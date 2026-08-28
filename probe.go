package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Probe は1ページだけ取得して、どのセレクタが何件マッチしたかを表で出す。
// セレクタが今も有効かを最小の負荷で確認するためのモード。
func Probe(cfg *Config, pref int, catKey, detailURL string) error {
	cat, ok := cfg.Category(catKey)
	if !ok {
		return fmt.Errorf("未知の分類: %s", catKey)
	}
	f := NewFetcher(cfg)
	s := cfg.Selectors

	listURL := cat.Host + "/" + fmt.Sprintf(cat.PathTmpl, pref)
	fmt.Printf("== 一覧ページ ==\n%s\n\n", listURL)

	body, err := f.Get(listURL)
	if err != nil {
		fmt.Printf("[NG] 取得できませんでした: %v\n", err)
		fmt.Println("→ URL体系が変わっている可能性があります。")
		return err
	}
	fmt.Printf("[OK] HTTP 200 / %d バイト\n\n", len(body))
	os.WriteFile("probe_list.html", []byte(body), 0644)
	fmt.Print("生HTMLを probe_list.html に保存しました。\n\n")

	fmt.Println("--- 一覧セレクタ ---")
	report("件数", body, s.ResultCount)
	report("店舗リンク", body, s.SalonLink)

	// serviceAreaCd の自動発見（新規掲載店モードで必要）
	fmt.Println("\n--- serviceAreaCd 発見 ---")
	saRe := regexp.MustCompile(`serviceAreaCd=([A-Za-z0-9]+)`)
	saSeen := map[string]int{}
	for _, m := range saRe.FindAllStringSubmatch(body, -1) {
		saSeen[m[1]]++
	}
	if len(saSeen) == 0 {
		fmt.Println("  [NG] 一覧HTMLに serviceAreaCd が見つかりません。")
		fmt.Println("       新規掲載店モードは使えません（全店舗モードは影響なし）。")
	} else {
		for k, v := range saSeen {
			fmt.Printf("  [OK] %s  (出現 %d 回)\n", k, v)
		}
		best, bn := "", 0
		for k, v := range saSeen {
			if v > bn {
				best, bn = k, v
			}
		}
		fmt.Printf("\n  selectors.json の service_area_cd に次を追加してください:\n")
		fmt.Printf("    \"service_area_cd\": { \"%02d\": \"%s\" }\n", pref, best)
	}

	n := ParseResultCount(body, s)
	links := ParseSalonLinks(body, cat.Host, s)
	fmt.Printf("\n判定: 総件数=%d / 抽出リンク=%d 件\n", n, len(links))
	if len(links) > 0 {
		fmt.Println("先頭3件:")
		for i, l := range links {
			if i >= 3 {
				break
			}
			fmt.Println("  ", l)
		}
	}

	// 詳細ページ
	target := detailURL
	if target == "" && len(links) > 0 {
		target = links[0]
	}
	if target == "" {
		fmt.Println("\n[NG] 詳細ページを試せません（リンク抽出が0件）。")
		fmt.Println("probe_list.html を開いて実際のクラス名を確認してください。")
		return nil
	}

	fmt.Printf("\n== 詳細ページ ==\n%s\n\n", target)
	dbody, err := f.Get(target)
	if err != nil {
		fmt.Printf("[NG] 取得できませんでした: %v\n", err)
		return err
	}
	fmt.Printf("[OK] HTTP 200 / %d バイト\n", len(dbody))
	os.WriteFile("probe_detail.html", []byte(dbody), 0644)
	fmt.Print("生HTMLを probe_detail.html に保存しました。\n\n")

	fmt.Println("--- JSON-LD ---")
	ld := ExtractSalonLD(dbody)
	if ld == nil {
		fmt.Println("  [NG] サロンの JSON-LD が見つかりません。テーブル解析にフォールバックします。")
	} else {
		fmt.Printf("  [OK] @type=%s\n", ld.Type)
		for _, kv := range [][2]string{
			{"name", ld.Name}, {"telephone", ld.Telephone},
			{"address", ld.AddressText()}, {"description", ld.Description},
			{"priceRange", ld.PriceRange},
		} {
			mark := "NG"
			if kv[1] != "" {
				mark = "OK"
			}
			fmt.Printf("    [%s] %-12s %s\n", mark, kv[0], truncate(kv[1], 45))
		}
		if ld.Geo != nil {
			fmt.Printf("    [OK] %-12s %v, %v\n", "geo", ld.Geo.Lat, ld.Geo.Lng)
		} else {
			fmt.Printf("    [NG] %-12s\n", "geo")
		}
		if ld.AggregateRating != nil {
			fmt.Printf("    [OK] %-12s %s点 / %s件\n", "rating",
				ldFloat(ld.AggregateRating.RatingValue),
				ldFloat(ld.AggregateRating.ReviewCount))
		} else {
			fmt.Printf("    [NG] %-12s\n", "rating")
		}
	}
	if svc, area := BreadcrumbArea(dbody); svc != "" || area != "" {
		fmt.Printf("  [OK] パンくず: %s / %s\n", svc, area)
	}

	fmt.Println("\n--- 詳細ページの <th> ラベル一覧（実物） ---")
	thRe := regexp.MustCompile(`<th[^>]*>([^<]{1,20})</th>`)
	thSeen := map[string]bool{}
	for _, m := range thRe.FindAllStringSubmatch(dbody, -1) {
		v := strings.TrimSpace(m[1])
		if v != "" && !thSeen[v] {
			thSeen[v] = true
			fmt.Printf("    %s\n", v)
		}
	}

	fmt.Println("\n--- 個別要素セレクタ（JSON-LD が無い場合の予備） ---")
	report("店舗名カナ", dbody, s.SalonKana)
	report("電話番号", dbody, s.Phone)

	fmt.Println("\n--- 詳細セレクタ（ラベル駆動テーブル） ---")
	keys := []string{"住所", "アクセス・道案内", "営業時間", "定休日",
		"支払い方法", "席数", "スタッフ数", "カット価格",
		"駐車場", "こだわり条件", "お店のホームページ", "備考", "その他"}
	okCnt := 0
	for _, k := range keys {
		lbl := s.TableLabels[k]
		v := tableCell(dbody, s.TableCellTmpl, lbl)
		mark := "NG"
		if v != "" {
			mark = "OK"
			okCnt++
		}
		fmt.Printf("  [%s] %-12s (label=%s) %s\n", mark, k, lbl, truncate(v, 45))
	}

	rec := ParseDetail(dbody, target, cat.Label, s)
	fmt.Printf("\n--- 組み立て結果 ---\n")
	for i, h := range CSVHeader {
		fmt.Printf("  %-16s : %s\n", h, truncate(rec.Row()[i], 55))
	}

	fmt.Printf("\n===== 判定 =====\n")
	fmt.Printf("テーブル項目: %d/%d 成功\n", okCnt, len(keys))
	if rec.Phone != "" {
		fmt.Printf("電話番号: %s（取得できています）\n", rec.Phone)
	} else {
		fmt.Println("電話番号: 取得できず。probe_detail.html を確認してください。")
	}
	filled := 0
	for _, v := range rec.Row() {
		if v != "" {
			filled++
		}
	}
	fmt.Printf("CSV列の充足: %d/%d\n", filled, len(CSVHeader))
	if filled >= len(CSVHeader)-4 {
		fmt.Println("→ 本取得に進んで問題ありません。")
	} else {
		fmt.Println("→ 空欄が多いので selectors.json の調整を推奨します。")
	}
	return nil
}

func report(name, body string, pats []string) {
	fmt.Printf("  %s:\n", name)
	for _, p := range pats {
		re, err := regexp.Compile(p)
		if err != nil {
			fmt.Printf("    [ERR] 正規表現が不正: %s\n", p)
			continue
		}
		n := len(re.FindAllString(body, -1))
		mark := "NG"
		if n > 0 {
			mark = "OK"
		}
		fmt.Printf("    [%s] %3d 件  %s\n", mark, n, truncate(p, 70))
	}
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
