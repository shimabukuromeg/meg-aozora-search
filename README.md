# meg-aozora-search

## Build

```bash
$ go build -o aozora-collector cmd/aozora-collector/main.go
$ go build -o aozora-search cmd/aozora-search/main.go
```

## Usage

```bash
# スクレイピング & テーブル作成 & データ登録
$ ./aozora-collector

# DB照会 & 検索
# 著者
$ ./aozora-search authors
000338 フランス アナトール
000879 芥川 竜之介
001085 イエイツ ウィリアム・バトラー
001086 ゴーチェ テオフィル
002016 ダ・ヴィンチ レオナルド

# ID 001085の著者の作品
$ ./aozora-search titles 001085
   44 春の心臓
 1128 「ケルトの薄明」より

# フルテキスト検索
$ ./aozora-search query [検索ワード]
000879    81: 木曽義仲論 (芥川 竜之介)
000879    37: 戯作三昧 (芥川 竜之介)
000879    38: 戯作三昧 (芥川 竜之介)
000879    31: 偸盗 (芥川 竜之介)
000879  3745: 澄江堂雑記 (芥川 竜之介)
000879 55721: 本所両国 (芥川 竜之介)
000879    48: 本所両国 (芥川 竜之介)
```
