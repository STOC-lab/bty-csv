package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

const version = "0.2.0"

var errNoGUI = errors.New("このプラットフォームではGUIを利用できません")

func writeCSV(path string, recs []Record) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// BOM付きUTF-8。Shift-JISだと丸数字・中韓文字が落ちるため。
	if _, err := f.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return err
	}
	w := csv.NewWriter(f)
	if err := w.Write(CSVHeader); err != nil {
		return err
	}
	for _, r := range recs {
		if err := w.Write(r.Row()); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func main() {
	// 引数なしで起動されたらGUI、引数ありならCLI。
	if len(os.Args) == 1 {
		cfg, err := LoadConfig("selectors.json")
		if err != nil {
			fatal(err)
		}
		if err := RunGUI(cfg, "work"); err != nil {
			fatal(err)
		}
		return
	}

	attachConsole() // -H windowsgui でもコンソールへ出力できるようにする
	runCLI()
}

func runCLI() {
	var (
		probe   = flag.Bool("probe", false, "セレクタ検証モード（1〜2ページだけ取得）")
		export  = flag.Bool("export", false, "取得済みデータからCSVを書き出すだけ")
		resume  = flag.Bool("resume", false, "前回の中断地点から再開")
		pref    = flag.Int("pref", 28, "都道府県コード 1-47（28=兵庫県）")
		cat     = flag.String("cat", "hair", "分類 hair|nail|eyelash|relax|esthe|clinic")
		filter  = flag.String("filter", "", "住所に含む文字列で絞り込み（例: 姫路市）")
		newOpen = flag.Bool("newopen", false, "新規掲載店のみ")
		out     = flag.String("out", "美容サロン一覧.csv", "出力CSVパス")
		workDir = flag.String("work", "work", "作業ディレクトリ")
		cfgPath = flag.String("config", "selectors.json", "セレクタ設定ファイル")
		detail  = flag.String("detail-url", "", "-probe で試す詳細ページURL")
		showVer = flag.Bool("version", false, "バージョン表示")
	)
	flag.Parse()

	if *showVer {
		fmt.Printf("btycsv %s\n", version)
		return
	}

	cfg, err := LoadConfig(*cfgPath)
	if err != nil {
		fatal(err)
	}

	if *probe {
		if err := Probe(cfg, *pref, *cat, *detail); err != nil {
			os.Exit(1)
		}
		return
	}

	store, err := OpenStore(*workDir)
	if err != nil {
		fatal(err)
	}
	defer store.Close()

	if *export {
		recs, err := store.Records()
		if err != nil {
			fatal(err)
		}
		if err := writeCSV(*out, recs); err != nil {
			fatal(err)
		}
		fmt.Printf("%d 件を %s に書き出しました。\n", len(recs), *out)
		return
	}

	var st *State
	if *resume {
		st, err = LoadState(*workDir)
		if err != nil {
			fatal(fmt.Errorf("再開できません（state.json が読めません）: %w", err))
		}
		fmt.Printf("再開: 残ジョブ %d / キュー %d 件\n\n", len(st.Jobs), len(st.Queue))
	} else {
		if *pref < 1 || *pref > 47 {
			fatal(errors.New("都道府県コードは 1-47 です"))
		}
		c, ok := cfg.Category(*cat)
		if !ok {
			fatal(fmt.Errorf("未知の分類: %s", *cat))
		}
		st = &State{
			Jobs:    []Job{{Pref: *pref, Category: *cat, NewOpen: *newOpen}},
			Filter:  *filter,
			OutPath: *out,
			Phase:   "list",
		}
		fmt.Printf("対象: %s / %s", Prefectures[*pref], c.Label)
		if *filter != "" {
			fmt.Printf(" / 住所フィルタ「%s」", *filter)
		}
		fmt.Printf("\n作業ディレクトリ: %s\n", *workDir)
	}
	st.SetDir(*workDir)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	stopped := false

	cr := NewCrawler(cfg, st, store)
	cr.Cancelled = func() bool {
		if stopped {
			return true
		}
		select {
		case <-sig:
			stopped = true
			return true
		default:
			return false
		}
	}

	runErr := cr.Run()

	recs, err := store.Records()
	if err != nil {
		fatal(err)
	}
	if len(recs) > 0 {
		if err := writeCSV(st.OutPath, recs); err != nil {
			fatal(err)
		}
		fmt.Printf("\n%d 件を %s に出力しました。\n", len(recs), st.OutPath)
	}
	if runErr != nil {
		fmt.Fprintf(os.Stderr, "\n[error] %v\n", runErr)
		os.Exit(1)
	}
	if stopped {
		fmt.Println("-resume で途中から再開できます。")
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
	os.Exit(1)
}
