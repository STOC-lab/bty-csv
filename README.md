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

## 2026-08-28 実測で判明したこと

VPSから兵庫県ヘアサロン（`pre28`）で実測した結果、
**一覧ページは市販ソフト解析時（2026-03）から変化なし、詳細ページは全面改修済み**だった。

### 生きているもの

- URL体系 `pre28/` … HTTP 200 / 兵庫県 2,836件
- 件数セレクタ `numberOfResult`
- 店舗リンク `<h3 class="slnName">` … 1ページ20件

### 変わったもの

| 項目 | 旧（2026-03） | 現行（2026-08） |
|---|---|---|
| テーブル構造 | `<p class="c-paragraph">ラベル</p></th><td class="table__cell">` | `<th class="w120">ラベル</th><td>` |
| アクセス | 「アクセス」「道案内」の2項目 | 「アクセス・道案内」に統合 |
| 席数 | 「設備数」 | 「席数」 |
| 支払い | 「クレジットカード」 | 「支払い方法」 |
| 責任者情報 | あり | 廃止 |
| その他 | なし | 新設 |
| 電話番号 | `contact-modal__phone-number` に数値 | **`/tel/` へのリンクのみ**（本文に数値なし） |

### 最大の収穫: JSON-LD

詳細ページに schema.org の JSON-LD が埋まっており、
**電話番号・住所・紹介文・価格帯・緯度経度・評価・口コミ数が正規化された形で取れる**。

```json
{"@type":"HairSalon","name":"オーロ 塚口店(ORO)",
 "telephone":"06-6422-8786",
 "address":"兵庫県尼崎市南塚口町１丁目１２ー５BROOKLYN SQUARE１F",
 "geo":{"latitude":34.7512628,"longitude":135.4142878},
 "aggregateRating":{"ratingValue":4.71,"reviewCount":510}}
```

電話番号が本文から消えたため `/tel/` への追加リクエストが必要かと思われたが、
JSON-LD に入っていたので**リクエスト数は増えない**。

これを受けて、パーサの優先順位を **JSON-LD > テーブル > 正規表現** に変更した。
JSON-LD は正規化済みでマークアップ改修に強く、正規表現依存を大幅に減らせる。
JSON-LD が無いページでもテーブルへ自動フォールバックする。

CSVには JSON-LD 由来の **緯度・経度・エリア**（パンくず由来）の3列を追加し、26列にした。
Google Maps 側のデータと突き合わせるときに使える。

### 未解決

- **カナ** … 詳細ページ本体には無く、`/tel/` ページにのみ存在する。
  取得するなら店舗あたり1リクエスト増える。現状は空欄。
- **serviceAreaCd** … 一覧HTMLから消えた。新規掲載店モードは現在使えない。
  ただしパンくずから `svcSB`（関西）/ `macBL`（西宮・伊丹・芦屋・尼崎）という
  エリア体系が判明したので、ここを起点に再構成できる見込み。
- **一覧1ページあたりの件数** … 20件で正しいことを実測で確認済み。
