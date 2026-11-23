# README.md と doc/ の誤りと古い記述を直す

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-readme-and-doc
- Polished: {YYYY-MM-DD}

## 目的

README.md と doc/ の記述を、現在の実装と手順に合わせる。

## 現状

- `doc/USE.md`
  - 「Go 1.21 以降が必要です」としているが、`go.mod` は `go 1.27.4` である。net/http への移行で使っている ServeMux のメソッド付きのパターンと `Request.Pattern` も、Go 1.21 では動かない。
  - Sora 側の設定の例の `audio_streaming_url` のポート (5890) が、`config_example.ini` の `listen_port` (48080) と一致しない。
  - 「AWS Transcribe」は正式名の「Amazon Transcribe」と異なる。「Google Speech To Text」と「GCP Speech-to-Text」が混在している。
  - Amazon Transcribe の例は `./bin/suzu`、Speech-to-Text の例は `./suzu` で、実行ファイルのパスが食い違っている。
  - 「HTTPS 設定無効にする」は助詞が抜けている。「この URL を Sora の audio_streaming_url を指定すると」は「を」が重なっている。文末の句点が抜けている箇所がある。
  - コードブロックに言語の指定がないものがある。`-service` がインラインコードになっていない。
- `doc/DEV.md`
  - リンクの文字列に絵文字がある (AGENTS.md の「絵文字を使わないこと」に反する)。
  - 見出しの「ライブラリロード」は「ライブリロード」の誤記である。air のリポジトリは移転している。
  - 「リリースビルド」の手順が `make` になっている。`make` は bin/ に開発用のバイナリを作る。リリース用は `make release` で dist/ に作る。
- `doc/GCP.md`
  - 「上限には 5 分の制限があります」は「上限」と「制限」が重なっている。URL がそのまま置かれている。
- `README.md`
  - 「無限リトライ対応」としているが、`max_retry` の既定値は 0 で、リトライ回数には上限がある。
  - 「GCP Speech to Text」と「Google Cloud Speech-to-Text」が混在している。
  - 著作権の表示が 2022-2024 のままになっている。
  - 「契約頂いている」は「契約いただいている」と書く。
- `NOTICE.md` の Pion の著作権の表示 (`Copyright (c) 2018`) が、取り込んだファイルの SPDX (`2023 The Pion community`) と一致しない。上流の LICENSE の現在の表示は確認できていない。

## 設計方針

- 上記の誤りを直し、製品名は正式名 (Amazon Transcribe、Google Cloud Speech-to-Text) にそろえる。
- 表記は shiguredo-doc の規約に合わせる。
- `NOTICE.md` は、上流の LICENSE を確認して表示を合わせる。

## 完了条件

- 上記の誤りと古い記述がない。
