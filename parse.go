package main

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
)

var (
	reTag     = regexp.MustCompile(`<[^>]*>`)
	reSpace   = regexp.MustCompile(`[ \t\r\n\x{3000}\x{00a0}]+`)
	reZip     = regexp.MustCompile(`〒?\s*(\d{3})-?(\d{4})`)
	reSlnCode = regexp.MustCompile(`/(slnH\w+)`)
	reDigits  = regexp.MustCompile(`[\d,]+`)
	reFloat   = regexp.MustCompile(`\d+\.\d+|\d+`)
)

// clean はタグを除去して空白を正規化する。
func clean(s string) string {
	s = reTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = reSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// firstMatch は複数パターンを順に試し、最初にマッチしたグループ1を返す。
func firstMatch(body string, pats []string) string {
	for _, p := range pats {
		re, err := regexp.Compile(p)
		if err != nil {
			continue
		}
		if m := re.FindStringSubmatch(body); m != nil && len(m) > 1 {
			if v := clean(m[1]); v != "" {
				return v
			}
		}
	}
	return ""
}

// allMatches は複数パターンすべてのマッチを集めて重複除去する。
func allMatches(body string, pats []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range pats {
		re, err := regexp.Compile(p)
		if err != nil {
			continue
		}
		for _, m := range re.FindAllStringSubmatch(body, -1) {
			if len(m) < 2 {
				continue
			}
			v := strings.TrimSpace(html.UnescapeString(m[1]))
			if v == "" || seen[v] {
				continue
			}
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// tableCell はラベル駆動でテーブルセルを引く。
func tableCell(body string, tmpls []string, label string) string {
	q := regexp.QuoteMeta(label)
	for _, t := range tmpls {
		pat := fmt.Sprintf(t, q)
		re, err := regexp.Compile(pat)
		if err != nil {
			continue
		}
		if m := re.FindStringSubmatch(body); m != nil && len(m) > 1 {
			if v := clean(m[1]); v != "" {
				return v
			}
		}
	}
	return ""
}

// Record は CSV 23列に対応する。
type Record struct {
	Code      string
	Name      string
	Kana      string
	Intro     string
	Condition string
	Category  string
	Phone     string
	Zip       string
	Address   string
	Access    string
	Hours     string
	Closed    string
	Payment   string
	Homepage  string
	CutPrice  string
	Seats     string
	StaffCnt  string
	Parking   string
	Features  string
	Note      string
	Rating    string
	ReviewCnt string
	URL       string
	Lat       string
	Lng       string
	Area      string
}

func (r Record) Row() []string {
	return []string{
		r.Code, r.Name, r.Kana, r.Intro, r.Condition, r.Category,
		r.Phone, r.Zip, r.Address, r.Access, r.Hours, r.Closed,
		r.Payment, r.Homepage, r.CutPrice, r.Seats, r.StaffCnt,
		r.Parking, r.Features, r.Note, r.Rating, r.ReviewCnt, r.URL,
		r.Lat, r.Lng, r.Area,
	}
}

var CSVHeader = []string{
	"店舗コード", "店舗名", "店舗名カナ", "紹介文", "条件等", "分類",
	"電話番号", "郵便番号", "住所", "アクセス・道案内", "営業時間", "定休日",
	"支払い方法", "お店のホームページ", "カット価格", "設備／席数", "スタッフ数",
	"駐車場", "こだわり条件", "備考", "サロン平均点", "口コミ数", "ページURL",
	"緯度", "経度", "エリア",
}

// ParseDetail は詳細ページHTMLからレコードを組み立てる。
//
// 優先順位は JSON-LD > テーブル > 正規表現。
// JSON-LD (schema.org) は正規化済みで、サイトのマークアップ改修に強い。
// テーブルは JSON-LD に無い項目（営業時間・定休日・席数など）を補う。
func ParseDetail(body, url, catLabel string, s Selectors) Record {
	L := s.TableLabels
	get := func(k string) string {
		lbl, ok := L[k]
		if !ok {
			return ""
		}
		return tableCell(body, s.TableCellTmpl, lbl)
	}
	// 先に埋まっている方を採用する
	pick := func(vs ...string) string {
		for _, v := range vs {
			if v != "" {
				return v
			}
		}
		return ""
	}

	ld := ExtractSalonLD(body)
	var ldName, ldPhone, ldAddr, ldDesc, ldPrice, ldRating, ldReview, lat, lng string
	if ld != nil {
		ldName = ld.Name
		ldPhone = ld.Telephone
		ldAddr = ld.AddressText()
		ldDesc = ld.Description
		ldPrice = ld.PriceRange
		if ld.Geo != nil {
			lat = strconv.FormatFloat(ld.Geo.Lat, 'f', -1, 64)
			lng = strconv.FormatFloat(ld.Geo.Lng, 'f', -1, 64)
		}
		if ld.AggregateRating != nil {
			ldRating = ldFloat(ld.AggregateRating.RatingValue)
			ldReview = ldFloat(ld.AggregateRating.ReviewCount)
		}
	}

	// 住所: JSON-LD 優先、無ければテーブル。郵便番号があれば分離する。
	addrRaw := pick(ldAddr, get("住所"))
	zip, addr := "", addrRaw
	if m := reZip.FindStringSubmatch(addrRaw); m != nil {
		zip = m[1] + "-" + m[2]
		addr = strings.TrimSpace(reZip.ReplaceAllString(addrRaw, ""))
	}

	// アクセス: 現行は「アクセス・道案内」に統合済み。旧2項目も拾う。
	access := get("アクセス・道案内")
	if access == "" {
		a, g := get("アクセス"), get("道案内")
		switch {
		case a != "" && g != "":
			access = a + " / " + g
		default:
			access = pick(a, g)
		}
	}

	code := ""
	if m := reSlnCode.FindStringSubmatch(url); m != nil {
		code = m[1]
	}

	// 評価: JSON-LD 優先、無ければ正規表現から数値を切り出す。
	rating := ldRating
	if rating == "" {
		if m := reFloat.FindString(firstMatch(body, s.Rating)); m != "" {
			rating = m
		}
	}
	rc := ldReview
	if rc == "" {
		if m := reDigits.FindString(firstMatch(body, s.ReviewCnt)); m != "" {
			rc = strings.ReplaceAll(m, ",", "")
		}
	}

	_, area := BreadcrumbArea(body)

	// 席数は「設備／席数」列へ。ラベルは現行「席数」、旧「設備数」。
	seats := pick(get("席数"), get("設備数"))

	return Record{
		Code:      code,
		Name:      pick(ldName, firstMatch(body, s.SalonName)),
		Kana:      firstMatch(body, s.SalonKana),
		Intro:     pick(ldDesc, firstMatch(body, s.Catchphrase)),
		Condition: get("その他"),
		Category:  catLabel,
		Phone:     pick(ldPhone, firstMatch(body, s.Phone)),
		Zip:       zip,
		Address:   addr,
		Access:    access,
		Hours:     get("営業時間"),
		Closed:    get("定休日"),
		Payment:   get("支払い方法"),
		Homepage:  get("お店のホームページ"),
		CutPrice:  pick(get("カット価格"), ldPrice),
		Seats:     seats,
		StaffCnt:  get("スタッフ数"),
		Parking:   get("駐車場"),
		Features:  get("こだわり条件"),
		Note:      get("備考"),
		Rating:    rating,
		ReviewCnt: rc,
		URL:       url,
		Lat:       lat,
		Lng:       lng,
		Area:      area,
	}
}

// ParseResultCount は一覧ページの総件数を返す。
func ParseResultCount(body string, s Selectors) int {
	v := firstMatch(body, s.ResultCount)
	v = strings.ReplaceAll(reDigits.FindString(v), ",", "")
	n, _ := strconv.Atoi(v)
	return n
}

// ParseSalonLinks は一覧ページから詳細URLを抽出し絶対URL化する。
func ParseSalonLinks(body, host string, s Selectors) []string {
	raw := allMatches(body, s.SalonLink)
	var out []string
	for _, u := range raw {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if strings.HasPrefix(u, "//") {
			u = "https:" + u
		} else if strings.HasPrefix(u, "/") {
			u = host + u
		} else if !strings.HasPrefix(u, "http") {
			continue
		}
		out = append(out, u)
	}
	return out
}
