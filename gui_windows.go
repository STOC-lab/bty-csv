//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/lxn/walk"
	dcl "github.com/lxn/walk/declarative"
)

// checkListModel は TableView をチェックボックス付きリストとして使うためのモデル。
type checkListModel struct {
	walk.TableModelBase
	items   []string
	checked []bool
}

func newCheckListModel(items []string, def bool) *checkListModel {
	m := &checkListModel{items: items, checked: make([]bool, len(items))}
	for i := range m.checked {
		m.checked[i] = def
	}
	return m
}

func (m *checkListModel) RowCount() int { return len(m.items) }

func (m *checkListModel) Value(row, col int) interface{} { return m.items[row] }

func (m *checkListModel) Checked(row int) bool { return m.checked[row] }

func (m *checkListModel) SetChecked(row int, checked bool) error {
	m.checked[row] = checked
	return nil
}

func (m *checkListModel) SetAll(v bool) {
	for i := range m.checked {
		m.checked[i] = v
	}
	m.PublishRowsReset()
}

func (m *checkListModel) Selected() []int {
	var out []int
	for i, c := range m.checked {
		if c {
			out = append(out, i)
		}
	}
	return out
}

type mainWin struct {
	mw       *walk.MainWindow
	catModel *checkListModel
	prefModel *checkListModel
	catAll   *walk.CheckBox
	prefAll  *walk.CheckBox
	scope    *walk.ComboBox
	filterEd *walk.LineEdit
	btnRun   *walk.PushButton
	btnClear *walk.PushButton
	btnCancel *walk.PushButton
	logBox   *walk.TextEdit
	status   *walk.StatusBarItem

	cfg      *Config
	running  atomic.Bool
	cancel   atomic.Bool
	catKeys  []string
	workDir  string
}

func RunGUI(cfg *Config, workDir string) error {
	w := &mainWin{cfg: cfg, workDir: workDir}

	var catLabels []string
	for _, c := range cfg.Categories {
		catLabels = append(catLabels, c.Label)
		w.catKeys = append(w.catKeys, c.Key)
	}
	w.catModel = newCheckListModel(catLabels, true)
	w.catModel.checked[len(catLabels)-1] = false // 美容クリニックは既定オフ

	w.prefModel = newCheckListModel(Prefectures[1:], false)
	w.prefModel.checked[27] = true // 兵庫県

	err := dcl.MainWindow{
		AssignTo: &w.mw,
		Title:    "美容サロン情報取得ソフト",
		MinSize:  dcl.Size{Width: 580, Height: 560},
		Size:     dcl.Size{Width: 620, Height: 640},
		Layout:   dcl.VBox{MarginsZero: false},
		StatusBarItems: []dcl.StatusBarItem{
			{AssignTo: &w.status, Text: "準備完了", Width: 560},
		},
		Children: []dcl.Widget{
			// 上段: マスタトグルと掲載範囲
			dcl.Composite{
				Layout: dcl.HBox{MarginsZero: true},
				Children: []dcl.Widget{
					dcl.CheckBox{
						AssignTo: &w.catAll,
						Text:     "美容サロン分類選択：",
						MaxSize:  dcl.Size{Width: 160},
						OnCheckedChanged: func() {
							w.catModel.SetAll(w.catAll.Checked())
						},
					},
					dcl.HSpacer{},
					dcl.CheckBox{
						AssignTo: &w.prefAll,
						Text:     "",
						MaxSize:  dcl.Size{Width: 20},
						OnCheckedChanged: func() {
							w.prefModel.SetAll(w.prefAll.Checked())
						},
					},
					dcl.ComboBox{
						AssignTo:     &w.scope,
						Model:        []string{"全店舗", "新規掲載店のみ"},
						CurrentIndex: 0,
						MinSize:      dcl.Size{Width: 220},
					},
				},
			},
			// 中段: 2つのチェックリスト
			dcl.Composite{
				Layout: dcl.HBox{MarginsZero: true},
				Children: []dcl.Widget{
					dcl.Composite{
						Layout:  dcl.VBox{MarginsZero: true},
						MaxSize: dcl.Size{Width: 280},
						Children: []dcl.Widget{
							dcl.TableView{
								Model:            w.catModel,
								CheckBoxes:       true,
								HeaderHidden:     true,
								ColumnsOrderable: false,
								MinSize:          dcl.Size{Height: 160},
								MaxSize:          dcl.Size{Height: 175},
								Columns:          []dcl.TableViewColumn{{Width: 250}},
							},
							dcl.Composite{
								Layout: dcl.HBox{MarginsZero: true},
								Children: []dcl.Widget{
									dcl.PushButton{
										AssignTo:  &w.btnRun,
										Text:      "取得 (&R)",
										MinSize:   dcl.Size{Height: 34},
										OnClicked: w.onRun,
									},
									dcl.PushButton{
										AssignTo:  &w.btnClear,
										Text:      "クリア (&N)",
										MinSize:   dcl.Size{Height: 34},
										OnClicked: w.onClear,
									},
								},
							},
							dcl.PushButton{
								AssignTo:  &w.btnCancel,
								Text:      "キャンセル (&C)",
								Enabled:   false,
								MinSize:   dcl.Size{Height: 34},
								OnClicked: w.onCancel,
							},
							dcl.Composite{
								Layout: dcl.HBox{MarginsZero: true},
								Children: []dcl.Widget{
									dcl.Label{Text: "住所絞り込み:"},
									dcl.LineEdit{
										AssignTo:    &w.filterEd,
										Text:        "姫路市",
										ToolTipText: "住所にこの文字列を含む店舗だけ出力します。空欄で絞り込みなし。",
									},
								},
							},
							dcl.VSpacer{},
						},
					},
					dcl.TableView{
						Model:            w.prefModel,
						CheckBoxes:       true,
						HeaderHidden:     true,
						ColumnsOrderable: false,
						Columns:          []dcl.TableViewColumn{{Width: 250}},
					},
				},
			},
			// 下段: ログ
			dcl.TextEdit{
				AssignTo: &w.logBox,
				ReadOnly: true,
				VScroll:  true,
				MinSize:  dcl.Size{Height: 150},
			},
		},
	}.Create()
	if err != nil {
		return err
	}

	if f, err := walk.NewFont("Yu Gothic UI", 9, 0); err == nil {
		w.mw.SetFont(f)
	}

	w.appendLog("準備完了。外部サイトへのアクセスは1接続ずつ、間隔をあけて行います。")
	if _, err := LoadState(workDir); err == nil {
		w.appendLog("前回の中断状態が見つかりました。「取得」で続きから再開します。")
	}

	w.mw.Run()
	return nil
}

func (w *mainWin) appendLog(s string) {
	w.logBox.AppendText(s + "\r\n")
}

// logAsync は別スレッドから安全にログを追記する。
func (w *mainWin) logAsync(s string) {
	w.mw.Synchronize(func() { w.appendLog(s) })
}

func (w *mainWin) setStatus(s string) {
	w.mw.Synchronize(func() { w.status.SetText(s) })
}

func (w *mainWin) setRunning(v bool) {
	w.running.Store(v)
	w.mw.Synchronize(func() {
		w.btnRun.SetEnabled(!v)
		w.btnClear.SetEnabled(!v)
		w.btnCancel.SetEnabled(v)
	})
}

func (w *mainWin) onClear() {
	w.catModel.SetAll(false)
	w.prefModel.SetAll(false)
	w.catAll.SetChecked(false)
	w.prefAll.SetChecked(false)
	w.scope.SetCurrentIndex(0)
	w.filterEd.SetText("")
	w.appendLog("抽出条件を初期状態に戻しました。")
}

func (w *mainWin) onCancel() {
	w.cancel.Store(true)
	w.setStatus("中断しています...")
	w.appendLog("中断要求を受け付けました。現在のページを終えたら停止します。")
}

func (w *mainWin) onRun() {
	if w.running.Load() {
		return
	}

	cats := w.catModel.Selected()
	prefs := w.prefModel.Selected()

	// 再開可能な state があるか確認
	resumable := false
	if st, err := LoadState(w.workDir); err == nil && st.Phase != "done" &&
		(st.Cur != nil || len(st.Jobs) > 0) {
		resumable = true
	}

	if !resumable {
		if len(cats) == 0 {
			walk.MsgBox(w.mw, "設定エラー",
				"美容サロン分類は1つ以上を選択してください。",
				walk.MsgBoxIconWarning)
			return
		}
		if len(prefs) == 0 {
			walk.MsgBox(w.mw, "設定エラー",
				"都道府県は1つ以上を選択してください。",
				walk.MsgBoxIconWarning)
			return
		}
	}

	newOpen := w.scope.CurrentIndex() == 1
	filter := w.filterEd.Text()

	var st *State
	if resumable {
		st, _ = LoadState(w.workDir)
		w.appendLog(fmt.Sprintf("前回の続きから再開します（残ジョブ %d / キュー %d）",
			len(st.Jobs), len(st.Queue)))
	} else {
		st = &State{Filter: filter, Phase: "list"}
		for _, pi := range prefs {
			for _, ci := range cats {
				st.Jobs = append(st.Jobs, Job{
					Pref: pi + 1, Category: w.catKeys[ci], NewOpen: newOpen,
				})
			}
		}
		st.OutPath = filepath.Join(".", fmt.Sprintf("美容サロン一覧_%s.csv",
			time.Now().Format("20060102_150405")))
		w.appendLog(fmt.Sprintf("取得を開始します（%d 都道府県 × %d 分類 = %d ジョブ）",
			len(prefs), len(cats), len(st.Jobs)))
		if filter != "" {
			w.appendLog(fmt.Sprintf("住所フィルタ: %s", filter))
		}
	}
	st.SetDir(w.workDir)

	w.cancel.Store(false)
	w.setRunning(true)

	go w.worker(st)
}

func (w *mainWin) worker(st *State) {
	defer w.setRunning(false)

	store, err := OpenStore(w.workDir)
	if err != nil {
		w.logAsync("エラー: " + err.Error())
		return
	}
	defer store.Close()

	cr := NewCrawler(w.cfg, st, store)
	cr.Log = w.logAsync
	cr.Cancelled = func() bool { return w.cancel.Load() }
	cr.Report = func(p Progress) {
		w.setStatus(fmt.Sprintf("%s  %s %d/%s  取得済み %d 件",
			p.JobLabel, p.Phase, p.Cur, pageLabel(p.Total), p.Collected))
	}

	runErr := cr.Run()

	recs, err := store.Records()
	if err == nil && len(recs) > 0 {
		out := st.OutPath
		if out == "" {
			out = "美容サロン一覧.csv"
		}
		if err := writeCSV(out, recs); err != nil {
			w.logAsync("CSV出力に失敗しました: " + err.Error())
		} else {
			abs, _ := filepath.Abs(out)
			w.logAsync(fmt.Sprintf("%d 件をCSVへ出力しました: %s", len(recs), abs))
			w.mw.Synchronize(func() {
				if walk.MsgBox(w.mw, "出力完了",
					fmt.Sprintf("%d 件をCSVへ出力しました。\n\n%s\n\n保存したファイルを開きますか？",
						len(recs), abs),
					walk.MsgBoxYesNo|walk.MsgBoxIconInformation) == walk.DlgCmdYes {
					openFile(abs)
				}
			})
		}
	}

	switch {
	case runErr != nil:
		w.logAsync("エラー: " + runErr.Error())
		w.setStatus("エラーで停止しました")
	case w.cancel.Load():
		w.setStatus("中断しました（「取得」で再開できます）")
	default:
		w.logAsync("店舗情報の取得が完了しました。")
		w.setStatus("完了")
		os.Remove(filepath.Join(w.workDir, "state.json"))
	}
}
