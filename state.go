package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
)

// Job は「1都道府県 × 1分類」の取得単位。
type Job struct {
	Pref     int    `json:"pref"`
	Category string `json:"category"`
	NewOpen  bool   `json:"new_open"`
}

// State は中断・再開のためのカーソル。1ページ処理ごとに atomic に書き出す。
type State struct {
	Jobs      []Job    `json:"jobs"` // 未着手のジョブ
	Cur       *Job     `json:"cur"`  // 実行中のジョブ
	Filter    string   `json:"filter"`
	OutPath   string   `json:"out_path"`
	Phase     string   `json:"phase"` // "list" | "detail" | "done"
	ListPage  int      `json:"list_page"`
	TotalPage int      `json:"total_page"`
	Queue     []string `json:"queue"`
	DoneCount int      `json:"done_count"`
	dir       string
}

func statePath(dir string) string  { return filepath.Join(dir, "state.json") }
func recordPath(dir string) string { return filepath.Join(dir, "records.ndjson") }
func seenPath(dir string) string   { return filepath.Join(dir, "seen.txt") }

func LoadState(dir string) (*State, error) {
	b, err := os.ReadFile(statePath(dir))
	if err != nil {
		return nil, err
	}
	s := &State{}
	if err := json.Unmarshal(b, s); err != nil {
		return nil, err
	}
	s.dir = dir
	return s, nil
}

// Save は tmp へ書いてから rename する（途中で落ちても壊れない）。
func (s *State) Save() error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := statePath(s.dir) + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, statePath(s.dir))
}

// NextJob は次のジョブを取り出してカレントにする。
func (s *State) NextJob() bool {
	if len(s.Jobs) == 0 {
		s.Cur = nil
		return false
	}
	j := s.Jobs[0]
	s.Jobs = s.Jobs[1:]
	s.Cur = &j
	s.Phase = "list"
	s.ListPage = 1
	s.TotalPage = 0
	s.Queue = nil
	return true
}

// Store は取得済みURLの集合とレコードの追記ログを持つ。
type Store struct {
	dir  string
	seen map[string]bool
	sf   *os.File
	rf   *os.File
}

func OpenStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	st := &Store{dir: dir, seen: map[string]bool{}}

	if f, err := os.Open(seenPath(dir)); err == nil {
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			if l := sc.Text(); l != "" {
				st.seen[l] = true
			}
		}
		f.Close()
	}

	var err error
	st.sf, err = os.OpenFile(seenPath(dir), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	st.rf, err = os.OpenFile(recordPath(dir), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		st.sf.Close()
		return nil, err
	}
	return st, nil
}

func (s *Store) Seen(url string) bool { return s.seen[url] }
func (s *Store) Count() int           { return len(s.seen) }

func (s *Store) Add(url string, r Record) error {
	if s.seen[url] {
		return nil
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if _, err := s.rf.Write(append(b, '\n')); err != nil {
		return err
	}
	if err := s.rf.Sync(); err != nil {
		return err
	}
	if _, err := s.sf.WriteString(url + "\n"); err != nil {
		return err
	}
	s.seen[url] = true
	return s.sf.Sync()
}

func (s *Store) Records() ([]Record, error) {
	f, err := os.Open(recordPath(s.dir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var out []Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			continue
		}
		out = append(out, r)
	}
	return out, sc.Err()
}

func (s *Store) Close() {
	if s.sf != nil {
		s.sf.Close()
	}
	if s.rf != nil {
		s.rf.Close()
	}
}

// SetDir は作業ディレクトリを設定する（LoadState 以外の経路用）。
func (s *State) SetDir(dir string) { s.dir = dir }
