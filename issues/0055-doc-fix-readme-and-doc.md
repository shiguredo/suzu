# README.md、doc/、NOTICE.md の誤りと古い記述を直す

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-readme-and-doc
- Polished: 2026-11-25

## 目的

README.md、doc/、NOTICE.md の記述を、現在の実装、手順、上流のライセンス表示に合わせる。

## 現状

- `doc/USE.md`
  - 「Go 1.21 以降が必要です」としているが、`go.mod` は `go 1.27.4` である。net/http への移行で使っている `http.ServeMux` のメソッド付きのパターン (`server.go` の `handleRoute` の `mux.Handle(method+" "+path, h)`) は Go 1.22 以降、`http.Request.Pattern` (`metrics.go` の `r.Pattern`) は Go 1.23 以降で追加されたため、Go 1.21 では動かない。
  - Sora 側の設定の例の `audio_streaming_url` のポート (5890) が、`config_example.ini` の `listen_port` (48080) と一致しない。
  - 「AWS Transcribe」は正式名の「Amazon Transcribe」と異なる。「Google Speech To Text」と「GCP Speech-to-Text」が混在している。正式名は「Amazon Transcribe」「Google Cloud Speech-to-Text」である。
  - Amazon Transcribe の例は `./bin/suzu`、Speech-to-Text の例は `./suzu` で、実行ファイルのパスが食い違っている。`make` は `bin/suzu` を作るため、両方とも `./bin/suzu` にそろえる。
  - 「HTTPS 設定無効にすることで」は「HTTPS 設定を無効にすることで」と書く。「この URL を Sora の audio_streaming_url を指定すると」は「この URL を Sora の `audio_streaming_url` に指定すると」と書く。文末の句点が抜けている箇所は、`**Go 1.21 以降が必要です**` と `config_example.ini をコピーして、AWS や GCP の設定をしてください` の 2 箇所である。
  - コードブロックに言語の指定がないものは、Amazon Transcribe と Google Cloud Speech-to-Text の実行例の 2 箇所である。`-service` がインラインコードになっていない箇所は、両方の節の「-service で ...」の 2 箇所である。
- `doc/DEV.md`
  - リンクの文字列に絵文字がある (AGENTS.md の「絵文字を使わないこと」に反する)。
  - 見出しの「ライブラリロード」は「ライブリロード」の誤記である。air のリポジトリは `https://github.com/air-verse/air` に移転している。
  - 「リリースビルド」の手順が `make` になっている。`make` は bin/ に開発用のバイナリを作る。リリース用は `make release` で dist/ に作る。
- `doc/GCP.md`
  - 「上限には 5 分の制限があります」は「上限」と「制限」が重なっている。URL がそのまま置かれている。
- `README.md`
  - 「無限リトライ対応」としているが、`max_retry` の既定値は 0 で、リトライ回数には上限がある。「指定した回数のリトライ対応」と書く。
  - 「GCP Speech to Text」と「Google Cloud Speech-to-Text」が混在している。正式名の「Google Cloud Speech-to-Text」にそろえる。
  - 著作権の表示が 2022-2024 のままになっている。2022-2026 にする。
  - 「契約頂いている」は「契約いただいている」と書く。
- `NOTICE.md`
  - Pion の著作権の表示 (`Copyright (c) 2018`) が、取り込んだファイルの SPDX (`2023 The Pion community`) と一致しない。
  - 取り込んだファイルは Pion WebRTC v4.0.1 のもので、v4.0.1 の LICENSE は `Copyright (c) 2023 The Pion community <https://pion.ly>`、取り込んだファイルの SPDX は `2023 The Pion community <https://pion.ly>` である。
  - `NOTICE.md` のリンクは master を指しており、master の LICENSE は 2026 になっている。取り込んだ版の表示に合わせる。

## 設計方針

- 上記の誤りを直し、製品名は正式名 (Amazon Transcribe、Google Cloud Speech-to-Text) にそろえる。
- 表記は shiguredo-doc の規約に合わせる。
- `doc/USE.md` の Go のバージョンは `go.mod` に合わせて `Go 1.27.4 以降` とする。
- `doc/USE.md` の Sora 側の設定例のポートは `config_example.ini` の `listen_port` に合わせて `48080` とする。
- `doc/USE.md` の Amazon Transcribe と Google Cloud Speech-to-Text の実行例は `./bin/suzu` にそろえ、コードブロックに `console` を指定する。`-service` はインラインコードにする。
- `doc/DEV.md` の air のリンクは `[air-verse/air: Live reload for Go apps](https://github.com/air-verse/air)` にし、リンクの文字列から絵文字を除く。見出しは「ライブリロード」にする。リリースビルドは `make release` と `dist/` に合わせる。
- `doc/GCP.md` の「Google Cloud Speech-to-Text のストリーミングリクエストの音声の長さの上限には 5 分の制限がありますのでご注意ください。」は「Google Cloud Speech-to-Text のストリーミングリクエストの音声の長さの上限は 5 分です。ご注意ください。」に直す。URL は `[割り当てと上限](https://cloud.google.com/speech-to-text/quotas?hl=ja)` のようにリンクにする。
- `NOTICE.md` のリンクは `https://github.com/pion/webrtc/blob/v4.0.1/LICENSE` にし、著作権の表示は、取り込んだファイルの SPDX と上流の v4.0.1 の LICENSE に合わせて `Copyright (c) 2023 The Pion community <https://pion.ly>` にする。

## 完了条件

- `README.md`、`doc/USE.md`、`doc/DEV.md`、`doc/GCP.md`、`NOTICE.md` に上記の誤りと古い記述がない。
- `doc/USE.md` のビルドに必要な Go のバージョン、Sora 側の設定例のポート、実行例のパスが実装と一致している。
- `README.md` の製品名、リトライの説明、著作権の表示が現在の実装と年に合っている。
- `NOTICE.md` のリンクと Pion の著作権の表示が、取り込んだ v4.0.1 の LICENSE と SPDX に合っている。
