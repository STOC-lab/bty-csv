package main

import (
	"fmt"
	"strings"
)

const perPage = 20 // 一覧1ページあたりの件数（実測で要確認）

// Progress は GUI へ進捗を返すための構造体。
type Progress struct {
	JobLabel  string // "兵庫県 / ヘアサロン"
	Phase     string
	Cur       int
	Total     int
	Collected int
}

type Crawler struct {
	cfg   *Config
	f     *Fetcher
	st    *State
	store *Store

	// Log はメッセージを1行出力する。GUI ではログ欄へ、CLI では標準出力へ。
	Log func(string)
	// Report は進捗を通知する。nil 可。
	Report func(Progress)
	// Cancelled は true を返した時点で安全に中断する。
	Cancelled func() bool
}

func NewCrawler(cfg *Config, st *State, store *Store) *Crawler {
	return &Crawler{
		cfg: cfg, f: NewFetcher(cfg), st: st, store: store,
		Log:       func(s string) { fmt.Println(s) },
		Cancelled: func() bool { return false },
	}
}

func (c *Crawler) logf(format string, a ...any) { c.Log(fmt.Sprintf(format, a...)) }

func (c *Crawler) report(phase string, cur, total int) {
	if c.Report == nil {
		return
	}
	label := ""
	if c.st.Cur != nil {
		cat, _ := c.cfg.Category(c.st.Cur.Category)
		label = fmt.Sprintf("%s / %s", Prefectures[c.st.Cur.Pref], cat.Label)
	}
	c.Report(Progress{label, phase, cur, total, c.store.Count()})
}

func (c *Crawler) curCat() Category {
	cat, _ := c.cfg.Category(c.st.Cur.Category)
	return cat
}

func (c *Crawler) listURL(page int) string {
	j := c.st.Cur
	cat := c.curCat()

	if j.NewOpen {
		sa := c.cfg.ServiceAreaCd[fmt.Sprintf("%02d", j.Pref)]
		if sa != "" && cat.SearchTmpl != "" {
			u := cat.Host + fmt.Sprintf(cat.SearchTmpl, sa)
			if page > 1 {
				u += fmt.Sprintf("&page=%d", page)
			}
			return u
		}
		// SAコード未設定。全店舗として続行する（呼び出し側で警告済み）。
	}

	base := cat.Host + "/" + fmt.Sprintf(cat.PathTmpl, j.Pref)
	if page <= 1 {
		return base
	}
	return base + fmt.Sprintf(cat.PageTmpl, page)
}

// Run は残ジョブを順に処理する。中断しても state から再開できる。
func (c *Crawler) Run() error {
	for {
		if c.st.Cur == nil {
			if !c.st.NextJob() {
				c.st.Phase = "done"
				return c.st.Save()
			}
			cat := c.curCat()
			c.logf("")
			c.logf("=== %s / %s （残ジョブ %d）===",
				Prefectures[c.st.Cur.Pref], cat.Label, len(c.st.Jobs))

			if c.st.Cur.NewOpen {
				sa := c.cfg.ServiceAreaCd[fmt.Sprintf("%02d", c.st.Cur.Pref)]
				if sa == "" || cat.SearchTmpl == "" {
					c.logf("  [警告] %s の serviceAreaCd が未設定のため、新規掲載店の絞り込みができません。",
						Prefectures[c.st.Cur.Pref])
					c.logf("         全店舗として取得します。-probe で SAコードを確認し、")
					c.logf("         selectors.json の service_area_cd に登録してください。")
				}
			}
			if err := c.st.Save(); err != nil {
				return err
			}
		}

		if c.st.Phase == "list" {
			if err := c.runList(); err != nil {
				return err
			}
		}
		if c.cancelled() {
			return c.st.Save()
		}
		if c.st.Phase == "detail" {
			if err := c.runDetail(); err != nil {
				return err
			}
		}
		if c.cancelled() {
			return c.st.Save()
		}
		c.st.Cur = nil
	}
}

func (c *Crawler) cancelled() bool {
	if c.Cancelled() {
		c.logf("")
		c.logf("[中断] 現在位置を保存しました。「再開」で続きから実行できます。")
		return true
	}
	return false
}

func (c *Crawler) runList() error {
	cat := c.curCat()
	if c.st.ListPage == 0 {
		c.st.ListPage = 1
	}

	for {
		if c.cancelled() {
			return c.st.Save()
		}

		u := c.listURL(c.st.ListPage)
		c.logf("[一覧 %d/%s] %s", c.st.ListPage, pageLabel(c.st.TotalPage), u)
		c.report("一覧", c.st.ListPage, c.st.TotalPage)

		body, err := c.f.Get(u)
		if err == ErrNotFound {
			break
		}
		if err != nil {
			c.logf("  [error] %v — この分類を打ち切ります", err)
			break
		}

		if c.st.ListPage == 1 {
			n := ParseResultCount(body, c.cfg.Selectors)
			if n == 0 {
				c.logf("  [警告] 件数を取得できません。セレクタがずれている可能性があります。")
			}
			c.st.TotalPage = (n + perPage - 1) / perPage
			c.logf("  総件数 %d 件 / 想定 %d ページ", n, c.st.TotalPage)
		}

		links := ParseSalonLinks(body, cat.Host, c.cfg.Selectors)
		if len(links) == 0 {
			c.logf("  リンク0件。一覧の終端と判断します。")
			break
		}

		added := 0
		inQ := map[string]bool{}
		for _, q := range c.st.Queue {
			inQ[q] = true
		}
		for _, l := range links {
			if !c.store.Seen(l) && !inQ[l] {
				c.st.Queue = append(c.st.Queue, l)
				inQ[l] = true
				added++
			}
		}
		c.logf("  %d 件抽出（新規 %d / キュー計 %d）", len(links), added, len(c.st.Queue))

		c.st.ListPage++
		if err := c.st.Save(); err != nil {
			return err
		}
		if c.st.TotalPage > 0 && c.st.ListPage > c.st.TotalPage {
			break
		}
	}

	c.st.Phase = "detail"
	c.logf("[一覧完了] 詳細取得対象 %d 件", len(c.st.Queue))
	return c.st.Save()
}

func (c *Crawler) runDetail() error {
	cat := c.curCat()
	total := len(c.st.Queue)
	done := 0

	for len(c.st.Queue) > 0 {
		if c.cancelled() {
			return c.st.Save()
		}

		u := c.st.Queue[0]
		if c.store.Seen(u) {
			c.st.Queue = c.st.Queue[1:]
			continue
		}
		done++
		c.report("詳細", done, total)

		body, err := c.f.Get(u)
		if err != nil {
			c.logf("  [%d/%d] %v — スキップ", done, total, err)
			c.st.Queue = c.st.Queue[1:]
			c.st.Save()
			continue
		}

		rec := ParseDetail(body, u, cat.Label, c.cfg.Selectors)

		if c.st.Filter != "" && !strings.Contains(rec.Address, c.st.Filter) {
			c.st.Queue = c.st.Queue[1:]
			c.st.Save()
			continue
		}
		if rec.Name == "" {
			c.logf("  [警告] 店舗名が取れません: %s", u)
		}
		if err := c.store.Add(u, rec); err != nil {
			return err
		}
		c.logf("  [%d/%d] %s / %s / %s", done, total, rec.Name, rec.Phone, rec.Address)

		c.st.Queue = c.st.Queue[1:]
		c.st.DoneCount++
		if err := c.st.Save(); err != nil {
			return err
		}
	}
	return c.st.Save()
}

func pageLabel(n int) string {
	if n <= 0 {
		return "?"
	}
	return fmt.Sprint(n)
}
