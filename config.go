package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Category は美容サロン分類ごとのURL体系を表す。
// bty_csv.exe のバイナリ解析で確認したパス構成をそのまま持つ。
type Category struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Host     string `json:"host"`      // https://beauty.hotpepper.jp
	PathTmpl string `json:"path_tmpl"` // "pre%02d/" や "g-nail/pre%02d/"
	PageTmpl string `json:"page_tmpl"` // "PN%d/" (パス付与) / "?page=%d" (クエリ)
	PageMode string `json:"page_mode"` // "path" | "query"
	// SearchTmpl は新規掲載店モードの検索URL。%s に serviceAreaCd が入る。
	SearchTmpl string `json:"search_tmpl"`
}

// Selectors は HTML 抽出パターン。サイト改修時はこのJSONだけ直せば済むようにする。
type Selectors struct {
	// 件数。複数書けて、最初にマッチしたものを使う。
	ResultCount []string `json:"result_count"`
	// 一覧ページからの店舗詳細リンク。
	SalonLink []string `json:"salon_link"`
	// 詳細ページ: ラベル駆動テーブル。%s にラベル名が入る。
	TableCellTmpl []string `json:"table_cell_tmpl"`
	// 詳細ページ: 個別要素。
	Phone      []string `json:"phone"`
	SalonName  []string `json:"salon_name"`
	SalonKana  []string `json:"salon_kana"`
	Catchphrase []string `json:"catchphrase"`
	Rating     []string `json:"rating"`
	ReviewCnt  []string `json:"review_count"`
	// 詳細ページで引くテーブルのラベル一覧（CSV列名 -> HTMLラベル）
	TableLabels map[string]string `json:"table_labels"`
}

type Config struct {
	Categories []Category `json:"categories"`
	Selectors  Selectors  `json:"selectors"`
	UserAgent  string     `json:"user_agent"`
	// レート制御
	MinDelayMS int `json:"min_delay_ms"`
	MaxDelayMS int `json:"max_delay_ms"`
	MaxRetries int `json:"max_retries"`
	TimeoutSec int `json:"timeout_sec"`
	// ServiceAreaCd は都道府県コード("28")→ serviceAreaCd の対応表。
	// 新規掲載店モードで必要。-probe が実際のHTMLから拾って教えてくれる。
	ServiceAreaCd map[string]string `json:"service_area_cd"`
}

func DefaultConfig() *Config {
	return &Config{
		Categories: []Category{
			{"hair", "ヘアサロン", "https://beauty.hotpepper.jp", "pre%02d/", "PN%d/", "path",
				"/CSP/bt/salonSearch/search/?serviceAreaCd=%s&freeword=NEWOPEN&searchGender=ALL&sortType=popular&fromSearchCondition=true&searchConditionPanelOpenFlg=1"},
			{"nail", "ネイルサロン", "https://beauty.hotpepper.jp", "g-nail/pre%02d/", "PN%d/", "path",
				"/CSP/kr/salonSearch/search/?serviceAreaCd=%s&freeword=NEWOPEN&searchGender=ALL&genreCd=GR04&genreAlias=nail"},
			{"eyelash", "まつげサロン", "https://beauty.hotpepper.jp", "g-eyelash/pre%02d/", "PN%d/", "path",
				"/CSP/kr/salonSearch/search/?serviceAreaCd=%s&freeword=NEWOPEN&searchGender=ALL&genreCd=GR05&genreAlias=nail"},
			{"relax", "リラクサロン", "https://beauty.hotpepper.jp", "relax/pre%02d/", "PN%d/", "path",
				"/CSP/kr/salonSearch/search/?serviceAreaCd=%s&freeword=NEWOPEN&searchGender=ALL&genreAlias=relax"},
			{"esthe", "エステサロン", "https://beauty.hotpepper.jp", "esthe/pre%02d/", "PN%d/", "path",
				"/CSP/kr/salonSearch/search/?serviceAreaCd=%s&freeword=NEWOPEN&searchGender=ALL&genreAlias=esthe"},
			{"clinic", "美容クリニック", "https://clinic.beauty.hotpepper.jp", "prefecture%02d/", "?page=%d", "query", ""},
		},
		ServiceAreaCd: map[string]string{},
		Selectors: Selectors{
			// --- bty_csv.exe から抽出した原文パターン（2026-03時点） ---
			ResultCount: []string{
				`(?s)<p><span class="numberOfResult">(.*?)</span>`,
				`(?s)<span class="numberOfResult">(.*?)</span>`,
				`(?s)<span class="c-search-result-heading__count">(.*?)</span>`,
			},
			SalonLink: []string{
				`(?s)<h3 class="slnName"><a href="(.*?)"`,
				`(?s)<h3 class="slcHead"><a href="(.*?)"`,
				`(?s)<p class="clinic__name"><a href="(.*?)"`,
			},
			TableCellTmpl: []string{
				`(?s)<p class="c-paragraph">%s</p></th><td class="table__cell"><p class="c-paragraph">(.*?)</p></td></tr>`,
				`(?s)<p class="c-paragraph">%s</p></th><td class="table__cell">(.*?)</td></tr>`,
				`(?s)<p class="c-paragraph">%s</p></th><td class="table__cell">(.*?)</table></td></tr>`,
			},
			Phone: []string{
				`(?s)<p class="contact-modal__phone-number">(.*?)</p>`,
			},
			SalonName: []string{
				`(?s)<p class="detailTitle"[^>]*>.*?<a[^>]*>(.*?)</a>`,
				`(?s)<h1[^>]*>(.*?)</h1>`,
			},
			SalonKana: []string{
				`(?s)<p class="fs10 mT5">(.*?)</p>`,
			},
			Catchphrase: []string{
				`(?s)<p class="clinic-overview__catchphrase">(.*?)</p>`,
				`(?s)<p class="mT10 fs14 b">(.*?)</p>`,
			},
			Rating: []string{
				`(?s)<span class="[^"]*ratingValue[^"]*">(.*?)</span>`,
			},
			ReviewCnt: []string{
				`(?s)<em class="[^"]*count[^"]*">(.*?)</em>`,
			},
			TableLabels: map[string]string{
				"住所":       "住所",
				"アクセス":     "アクセス",
				"道案内":      "道案内",
				"駐車場":      "駐車場",
				"営業時間":     "営業時間",
				"定休日":      "定休日",
				"支払い方法":    "クレジットカード",
				"設備／席数":    "設備数",
				"スタッフ数":    "スタッフ数",
				"カット価格":    "カット価格",
				"こだわり条件":   "こだわり条件",
				"お店のホームページ": "お店のホームページ",
				"備考":       "備考",
				"責任者情報":    "責任者情報",
			},
		},
		UserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36",
		MinDelayMS: 2500,
		MaxDelayMS: 5000,
		MaxRetries: 3,
		TimeoutSec: 30,
	}
}

func LoadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			c := DefaultConfig()
			if err := SaveConfig(path, c); err != nil {
				return nil, err
			}
			fmt.Fprintf(os.Stderr, "[info] %s を既定値で作成しました\n", path)
			return c, nil
		}
		return nil, err
	}
	c := DefaultConfig()
	if err := json.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("%s の解析に失敗: %w", path, err)
	}
	return c, nil
}

func SaveConfig(path string, c *Config) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

func (c *Config) Category(key string) (Category, bool) {
	for _, cat := range c.Categories {
		if cat.Key == key {
			return cat, true
		}
	}
	return Category{}, false
}

var Prefectures = []string{
	"", "北海道", "青森県", "岩手県", "宮城県", "秋田県", "山形県", "福島県",
	"茨城県", "栃木県", "群馬県", "埼玉県", "千葉県", "東京都", "神奈川県",
	"新潟県", "富山県", "石川県", "福井県", "山梨県", "長野県", "岐阜県",
	"静岡県", "愛知県", "三重県", "滋賀県", "京都府", "大阪府", "兵庫県",
	"奈良県", "和歌山県", "鳥取県", "島根県", "岡山県", "広島県", "山口県",
	"徳島県", "香川県", "愛媛県", "高知県", "福岡県", "佐賀県", "長崎県",
	"熊本県", "大分県", "宮崎県", "鹿児島県", "沖縄県",
}
