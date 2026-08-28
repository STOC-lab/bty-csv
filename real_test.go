package main

import "testing"

// 2026-08-28 に実測した ORO 塚口店の詳細ページ構造をそのまま使う。
const realDetail = `<html><head>
<script type="application/ld+json">
{"@context":"http://schema.org/","@type":"HairSalon",
"name":"オーロ 塚口店(ORO)",
"description":"《OROは外部講師として全国で活躍》 業界誌にも多数取り上げられる実力派サロン☆完全個室もご用意☆",
"priceRange":"￥3,850～","telephone":"06-6422-8786",
"address":"兵庫県尼崎市南塚口町１丁目１２ー５BROOKLYN SQUARE１F",
"geo":{"@type":"GeoCoordinates","latitude":34.7512628,"longitude":135.4142878},
"aggregateRating":{"@type":"AggregateRating","ratingValue":4.71,"reviewCount":510}}
</script>
<script type="application/ld+json">
{"@context":"http://schema.org","@type":"BreadcrumbList","itemListElement":[
{"@type":"ListItem","position":3,"item":{"@id":"https://beauty.hotpepper.jp/svcSB/","name":"関西トップ"}},
{"@type":"ListItem","position":4,"item":{"@id":"https://beauty.hotpepper.jp/svcSB/macBL/","name":"西宮・伊丹・芦屋・尼崎トップ"}}]}
</script>
<script type="application/ld+json">
{"@context":"http://schema.org/","@type":"BeautySalon",
"address":{"@type":"PostalAddress","name":"兵庫県尼崎市南塚口町１丁目１２ー５BROOKLYN SQUARE１F"},
"name":"オーロ 塚口店(ORO)","telephone":"06-6422-8786",
"url":"https://beauty.hotpepper.jp/slnH000743351/"}
</script></head><body>
<table class="slnDataTbl bdCell bgThNml fgThNml vaThT pCellV10H12 mT20">
<tr><th class="w120">電話番号</th><td colspan="3" class="w618"><a href="https://beauty.hotpepper.jp/slnH000743351/tel/">番号を表示</a></td></tr>
<tr><th class="w120">住所</th><td colspan="3" class="w618">兵庫県尼崎市南塚口町１丁目１２ー５BROOKLYN SQUARE１F&nbsp;</td></tr>
<tr><th class="w120">アクセス・道案内</th><td colspan="3" class="w618">阪急塚口駅【南口】からロータリーを超えて右に【3分】ほど歩くと左手に御座います。&nbsp;</td></tr>
<tr><th class="w120">営業時間</th><td colspan="3" class="w618">10：00～20：00（カット最終受付19：30）&nbsp;</td></tr>
<tr><th class="w120">定休日</th><td colspan="3" class="w618">毎週月曜（祝日の場合は営業、翌平日代休）&nbsp;</td></tr>
<tr><th class="w120">支払い方法</th><td colspan="3" class="w618">VISA MasterCard&nbsp;</td></tr>
<tr><th class="w120">席数</th><td class="w208 vaT">セット面8席&nbsp;</td></tr>
<tr><th class="w120">スタッフ数</th><td class="w208 vaT">12人&nbsp;</td></tr>
<tr><th class="w120">カット価格</th><td class="w208 vaT">￥3,850&nbsp;</td></tr>
<tr><th class="w120">駐車場</th><td class="w208 vaT">近隣にパーキングあり&nbsp;</td></tr>
<tr><th class="w120">こだわり条件</th><td colspan="3" class="w620">夜19時以降も受付OK／ヘアセット／禁煙&nbsp;</td></tr>
<tr><th class="w120">お店のホームページ</th><td colspan="3" class="w618">https://oro-hair.com/&nbsp;</td></tr>
<tr><th class="w120">その他</th><td colspan="3" class="w618">完全個室あり&nbsp;</td></tr>
<tr><th class="w120">備考</th><td colspan="3" class="w618">駐車場補助あり&nbsp;</td></tr>
</table>
<table><tr><th>初来店</th><td>￥3,850</td></tr><tr><th>2回目以降来店</th><td>￥4,000</td></tr></table>
</body></html>`

func TestRealDetail(t *testing.T) {
	cfg := DefaultConfig()
	r := ParseDetail(realDetail, "https://beauty.hotpepper.jp/slnH000743351/?cstt=1", "ヘアサロン", cfg.Selectors)

	want := map[string]string{
		"Code":      "slnH000743351",
		"Name":      "オーロ 塚口店(ORO)",
		"Phone":     "06-6422-8786",
		"Address":   "兵庫県尼崎市南塚口町１丁目１２ー５BROOKLYN SQUARE１F",
		"Access":    "阪急塚口駅【南口】からロータリーを超えて右に【3分】ほど歩くと左手に御座います。",
		"Hours":     "10：00～20：00（カット最終受付19：30）",
		"Closed":    "毎週月曜（祝日の場合は営業、翌平日代休）",
		"Payment":   "VISA MasterCard",
		"Seats":     "セット面8席",
		"StaffCnt":  "12人",
		"CutPrice":  "￥3,850",
		"Parking":   "近隣にパーキングあり",
		"Features":  "夜19時以降も受付OK／ヘアセット／禁煙",
		"Homepage":  "https://oro-hair.com/",
		"Condition": "完全個室あり",
		"Note":      "駐車場補助あり",
		"Rating":    "4.71",
		"ReviewCnt": "510",
		"Lat":       "34.7512628",
		"Lng":       "135.4142878",
		"Area":      "西宮・伊丹・芦屋・尼崎",
	}
	got := map[string]string{
		"Code": r.Code, "Name": r.Name, "Phone": r.Phone, "Address": r.Address,
		"Access": r.Access, "Hours": r.Hours, "Closed": r.Closed, "Payment": r.Payment,
		"Seats": r.Seats, "StaffCnt": r.StaffCnt, "CutPrice": r.CutPrice,
		"Parking": r.Parking, "Features": r.Features, "Homepage": r.Homepage,
		"Condition": r.Condition, "Note": r.Note, "Rating": r.Rating,
		"ReviewCnt": r.ReviewCnt, "Lat": r.Lat, "Lng": r.Lng, "Area": r.Area,
	}
	ok := 0
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%-10s got %q want %q", k, got[k], w)
		} else {
			ok++
		}
	}
	if r.Intro == "" {
		t.Error("Intro が空")
	}
	t.Logf("一致 %d/%d", ok, len(want))
	t.Logf("Intro = %q", r.Intro)

	if len(r.Row()) != len(CSVHeader) {
		t.Fatalf("列数 %d != ヘッダ %d", len(r.Row()), len(CSVHeader))
	}
	// 電話番号セルはリンクなので、テーブルからは "番号を表示" が返る。
	// JSON-LD が優先されていることの確認。
	if r.Phone == "番号を表示" {
		t.Error("JSON-LD ではなくテーブルを拾っている")
	}
}

// JSON-LD が無いページでもテーブルへフォールバックすること
func TestFallbackNoLD(t *testing.T) {
	cfg := DefaultConfig()
	h := `<h1>テストサロン</h1><table>
<tr><th class="w120">住所</th><td colspan="3" class="w618">〒670-0012 兵庫県姫路市本町68&nbsp;</td></tr>
<tr><th class="w120">営業時間</th><td colspan="3" class="w618">9:00～19:00&nbsp;</td></tr></table>`
	r := ParseDetail(h, "https://beauty.hotpepper.jp/slnH999/", "ヘアサロン", cfg.Selectors)
	if r.Name != "テストサロン" {
		t.Errorf("Name %q", r.Name)
	}
	if r.Zip != "670-0012" || r.Address != "兵庫県姫路市本町68" {
		t.Errorf("郵便番号分離 %q / %q", r.Zip, r.Address)
	}
	if r.Hours != "9:00～19:00" {
		t.Errorf("Hours %q", r.Hours)
	}
}

// 実測した一覧ページの形
func TestRealList(t *testing.T) {
	cfg := DefaultConfig()
	h := `<p><span class="numberOfResult">2836</span>件</p>
<h3 class="slnName"><a href="https://beauty.hotpepper.jp/slnH000743351/?cstt=1">A</a></h3>
<h3 class="slnName"><a href="https://beauty.hotpepper.jp/slnH000265401/?cstt=2">B</a></h3>`
	if n := ParseResultCount(h, cfg.Selectors); n != 2836 {
		t.Fatalf("件数 %d want 2836", n)
	}
	l := ParseSalonLinks(h, "https://beauty.hotpepper.jp", cfg.Selectors)
	if len(l) != 2 {
		t.Fatalf("リンク %d: %v", len(l), l)
	}
}
