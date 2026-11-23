# Ogg ファイルの出力で、リクエストヘッダーの値によって ogg_dir の外にファイルを作成できる

- Created: 2026-11-17
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-ogg-file-path-traversal
- Polished: 2026-11-23

## 目的

`enable_ogg_file_output` が有効なとき、Ogg ファイルを `ogg_dir` の中にだけ作成するようにする。現状はリクエストヘッダーの値をそのままファイル名に使うため、値に `../` などを含めると、`ogg_dir` の外に末尾が `.ogg` のファイルを作成・上書きできる。

## 現状

- `handler.go` の `opus2ogg` は、`EnableOggFileOutput` が有効なとき、`fmt.Sprintf("%s-%s.ogg", header.SoraSessionID, header.SoraConnectionID)` でファイル名を組み立て、`path.Join(c.OggDir, fileName)` のパスを `os.Create` で開く。
- `header.SoraSessionID` と `header.SoraConnectionID` は、`newSoraHeader` がリクエストヘッダーの `sora-session-id` と `sora-connection-id` から取り出した値で、どこでも検証していない。
- `path.Join` はパスを正規化するため、値に `../` を含めると `ogg_dir` より上のディレクトリを指す。`/` を含めるとサブディレクトリを指す。
- `os.Create` は既存のファイルを切り詰めて開くため、末尾が `-<sora-connection-id>.ogg` になる既存のファイルを上書きできる。
- Suzu は、`https = true` かつ `tls_verify_cacert_path` を設定している場合を除き、待ち受けに到達できる誰からでもリクエストを受け付ける。
- `opus2ogg` を呼ぶのは `AmazonTranscribeV2Handler.Handle` と `SpeechToTextHandler.Handle` で、`-service aws`、`awsv2`、`gcp` が該当する。`awsv2` は `aws` と同じハンドラーに登録されている。
- `opus2ogg` は音声を受信する前に Ogg ファイルを作成するため、外部サービスに接続しなくてもファイルは作成される。
- `enable_ogg_file_output` の既定値は false。

### 再現手順

1. `enable_ogg_file_output = true` と、`ogg_dir` に任意のディレクトリを設定し、`-service aws` で起動する。
2. HTTP/2 で `POST /speech` に、`sora-session-id: ../escaped`、`sora-connection-id: x`、`sora-audio-streaming-language-code: ja-JP` のヘッダーを付けて、空のボディで送る。
3. `ogg_dir` の親ディレクトリに `escaped-x.ogg` が作成される。

## 設計方針

- `opus2ogg` で、`EnableOggFileOutput` が有効な場合に限り、Ogg ファイル名に使う `sora-session-id` と `sora-connection-id` の値が、パスの区切り文字 (`/` と `\`) と `..` を含まないことを検証する。
- 検証に失敗した場合は Ogg ファイルを作成せず、`os.Create` が失敗した場合と同じく `nil, err` を返す。
- `sora-session-id` と `sora-connection-id` のそれぞれに不正な値を含むリクエストを送り、Ogg ファイルが作成されず、エラーが返ることを確認するテストを追加する。

## 完了条件

- `enable_ogg_file_output = true` のとき、`sora-session-id` または `sora-connection-id` にパスの区切り文字や `..` を含むリクエストで、Ogg ファイルが作成されず、エラーが返る。
- 上記を確認するテストがある。
