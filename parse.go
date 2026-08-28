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
	reSpace   = regexp.MustCompile(`[ \t\r\n\x{3000}]+`)
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
}

func (r Record) Row() []string {
	return []string{
		r.Code, r.Name, r.Kana, r.Intro, r.Condition, r.Category,
		r.Phone, r.Zip, r.Address, r.Access, r.Hours, r.Closed,
		r.Payment, r.Homepage, r.CutPrice, r.Seats, r.StaffCnt,
		r.Parking, r.Features, r.Note, r.Rating, r.ReviewCnt, r.URL,
	}
}

var CSVHeader = []string{
	"店舗コード", "店舗名", "店舗名カナ", "紹介文", "条件等", "分類",
	"電話番号", "郵便番号", "住所", "アクセス・道案内", "営業時間", "定休日",
	"支払い方法", "お店のホームページ", "カット価格", "設備／席数", "スタッフ数",
	"駐車場", "こだわり条件", "備考", "サロン平均点", "口コミ数", "ページURL",
}

// ParseDetail は詳細ページHTMLからレコードを組み立てる。
func ParseDetail(body, url, catLabel string, s Selectors) Record {
	L := s.TableLabels
	get := func(k string) string {
		lbl, ok := L[k]
		if !ok {
			return ""
		}
		return tableCell(body, s.TableCellTmpl, lbl)
	}

	addrRaw := get("住所")
	zip := ""
	addr := addrRaw
	if m := reZip.FindStringSubmatch(addrRaw); m != nil {
		zip = m[1] + "-" + m[2]
		addr = strings.TrimSpace(reZip.ReplaceAllString(addrRaw, ""))
	}

	access := get("アクセス")
	if g := get("道案内"); g != "" {
		if access != "" {
			access += " / " + g
		} else {
			access = g
		}
	}

	code := ""
	if m := reSlnCode.FindStringSubmatch(url); m != nil {
		code = m[1]
	}

	rating := firstMatch(body, s.Rating)
	if m := reFloat.FindString(rating); m != "" {
		rating = m
	}
	rc := firstMatch(body, s.ReviewCnt)
	if m := reDigits.FindString(rc); m != "" {
		rc = strings.ReplaceAll(m, ",", "")
	}

	return Record{
		Code:      code,
		Name:      firstMatch(body, s.SalonName),
		Kana:      firstMatch(body, s.SalonKana),
		Intro:     firstMatch(body, s.Catchphrase),
		Condition: get("こだわり条件"),
		Category:  catLabel,
		Phone:     firstMatch(body, s.Phone),
		Zip:       zip,
		Address:   addr,
		Access:    access,
		Hours:     get("営業時間"),
		Closed:    get("定休日"),
		Payment:   get("支払い方法"),
		Homepage:  get("お店のホームページ"),
		CutPrice:  get("カット価格"),
		Seats:     get("設備／席数"),
		StaffCnt:  get("スタッフ数"),
		Parking:   get("駐車場"),
		Features:  get("こだわり条件"),
		Note:      get("備考"),
		Rating:    rating,
		ReviewCnt: rc,
		URL:       url,
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
