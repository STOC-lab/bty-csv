# bty-csv — 美容サロン情報取得ツール

ホットペッパービューティーの公開一覧ページから店舗情報を収集し、
23列のCSVへ出力する自社製ツール。

Windows では GUI 付き単一EXE、Linux (VPS) では CLI として動く。
外部依存は GUI 部分の `lxn/walk` のみで、収集エンジンは標準ライブラリだけで書いてある。

---

## セットアップ

### VPS (Linux)

```sh
apt-get update && apt-get install -y golang-go git
git clone https://github.com/STOC-lab/bty-csv.git
cd bty-csv
./build.sh
```

`btycsv_linux` ができる。SHA-256 が表示されるので控えておくとよい。

Go のバージョンが 1.22 未満だとビルドできない。その場合は公式バイナリを入れる。

```sh
go version   # go1.22 以上であること
```

### Windows

```bat
git clone https://github.com/STOC-lab/bty-csv.git
cd bty-csv
build.bat
```

`bty_csv.exe` ができる。`build.bat` は `rsrc` を自動で入れて
`app.manifest` を埋め込む。このマニフェスト（Common Controls 6.0 + DPI対応）が
無いと GUI が起動しないので、手でビルドする場合も忘れないこと。

---

## 使い方

### 1. まずセレクタ検証（必ず最初に実行する）

```sh
./btycsv_linux -probe -pref 28 -cat hair
```

一覧1ページと詳細1ページだけを取得し、セレクタごとに `[OK]` / `[NG]` と
マッチ件数を表示する。生HTMLは `probe_list.html` / `probe_detail.html` に残る。
`serviceAreaCd` も自動で探して表示する。

`[NG]` が出た項目は `selectors.json` を直す。再ビルドは不要。

### 2. 本取得

```sh
./btycsv_linux -pref 28 -cat hair -filter 姫路市 -out 姫路.csv
```

長時間かかるのでバックグラウンドで流す。

```sh
nohup ./btycsv_linux -pref 28 -cat hair -filter 姫路市 -out 姫路.csv > run.log 2>&1 &
tail -f run.log      # Ctrl+C で抜けても取得は続く
```

### 3. 中断・再開

```sh
pkill btycsv_linux           # 中断
./btycsv_linux -resume       # 続きから
```

1ページ処理するごとに `work/state.json` を一時ファイル経由で置き換えている。
途中で電源が落ちても壊れない。

### 4. CSVだけ作り直す

```sh
./btycsv_linux -export -out 再出力.csv
```

取得済みレコードは `work/records.ndjson` に追記されているので、
CSVは何度でも作り直せる。

### Windows GUI

`bty_csv.exe` を引数なしで起動すると GUI。
左が分類6種、右が都道府県47のチェックリスト。
引数を付けると CLI として動く（`AttachConsole` で親コンソールへ出力を戻す）。

---

## オプション

| オプション | 説明 |
|---|---|
| `-probe` | セレクタ検証モード |
| `-pref N` | 都道府県コード 1-47（28=兵庫県） |
| `-cat KEY` | `hair` `nail` `eyelash` `relax` `esthe` `clinic` |
| `-filter STR` | 住所の部分一致フィルタ（例: 姫路市） |
| `-newopen` | 新規掲載店のみ |
| `-out PATH` | 出力CSV（BOM付きUTF-8・23列） |
| `-resume` | 前回の中断地点から再開 |
| `-export` | 取得済みデータからCSVを書き出すだけ |
| `-work DIR` | 作業ディレクトリ（既定 `work`） |
| `-config PATH` | セレクタ設定（既定 `selectors.json`） |

---

## URL体系

既存の市販ソフト `bty_csv.exe`（シルクスクリプト製 Ver 2.2.7）を
バイナリ解析して復元したもの。2026年3月時点の仕様。

| 分類 | 一覧URL |
|---|---|
| ヘアサロン | `beauty.hotpepper.jp/pre{NN}/` |
| ネイルサロン | `beauty.hotpepper.jp/g-nail/pre{NN}/` |
| まつげサロン | `beauty.hotpepper.jp/g-eyelash/pre{NN}/` |
| リラクサロン | `beauty.hotpepper.jp/relax/pre{NN}/` |
| エステサロン | `beauty.hotpepper.jp/esthe/pre{NN}/` |
| 美容クリニック | `clinic.beauty.hotpepper.jp/prefecture{NN}/` |

`{NN}` は都道府県コードのゼロ埋め2桁。
ページ送りは前者が `PN{n}/`、クリニックのみ `?page={n}`。

新規掲載店モードは `salonSearch/search/` に `freeword=NEWOPEN` を投げる方式。
`serviceAreaCd` が必要で、この値は `-probe` で発見して
`selectors.json` の `service_area_cd` に登録する。

---

## レート制御

同時1接続、リクエスト間隔 2.5〜5.0秒（ジッター付き）。
429 / 503 は `Retry-After` を尊重して指数バックオフ。

`selectors.json` の `min_delay_ms` / `max_delay_ms` で変更できるが、短くしないこと。

---

## 設計メモ

- **収集エンジンは標準ライブラリのみ。** `lxn/walk` は `//go:build windows` の
  GUI ファイルからしか参照していないので、Linux ビルドには影響しない。
- **セレクタは全て `selectors.json` に外出し。** サイト改修時はJSONを直すだけで、
  再ビルドは不要。初回起動時に既定値で自動生成される。
- **一覧フェーズと詳細フェーズを分離。** 一覧で店舗URLだけ集めてキューに積み、
  詳細は別フェーズで処理する。どちらの途中でも再開できる。
- **`records.ndjson` は追記のみ。** CSVは常にここから生成する。

## 未検証事項

以下は2026年8月時点で未確認。`-probe` の結果で確定させること。

1. `pre{NN}/` と `PN{n}/` のURL体系が現在も有効か
2. 電話番号が `contact-modal__phone-number` として HTML に直接含まれるか
   （JSレンダリングに変わっていたら設計変更が必要）
3. 一覧1ページあたりの件数（20と仮定。違えば `crawl.go` の `perPage` を修正）
4. 店舗名・カナ・平均点・口コミ数のセレクタ
   （バイナリに明示パターンが無く推定で置いた箇所。`[NG]` になる可能性が最も高い）
