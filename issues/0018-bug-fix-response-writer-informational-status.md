# 1xx のステータスコードを送信済みのステータスコードとして記録する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-response-writer-informational-status
- Polished: {YYYY-MM-DD}

## 目的

アクセスログの status とメトリクスの code が、1xx を書き込んだ場合も、クライアントが実際に受け取ったステータスコードになるようにする。

## 現状

- `middleware.go` の `responseWriter.WriteHeader` は、委譲した後に、渡されたステータスコードを送信済みとして記録する。コメントでは「1xx の情報レスポンスは扱わない (Suzu では送信しない)」としている。
- net/http は、101 以外の 1xx を情報レスポンスとして送り、ヘッダーを送信済みにしない。その後の `WriteHeader` や `Write` で、最終的なステータスコードを送る。HTTP/2 でも同じ。
- そのため 1xx を書き込むと、ラッパーは送信済みと判断し、その後の `WriteHeader` を無視する。アダプター (`errorHandlerFunc`) もエラー応答を書かない。
  - 実測では、`WriteHeader(103)` の後にハンドラーがエラーを返すと、クライアントには 200 の空のボディが届き、アクセスログの status は 103 になった。
- `handler.go` の `createSpeechHandler` は、`SuzuError.Code` を 100 から 999 の範囲であれば `WriteHeader` に渡すため、`SuzuError.Code` が 1xx の場合にこの経路に入る。Amazon Transcribe の HTTP ステータスコードが 1xx になることは実質ない。

## 設計方針

- `responseWriter.WriteHeader` は、101 以外の 1xx を委譲するが、送信済みとして記録しない。
- `createSpeechHandler` は、`SuzuError.Code` が 200 未満の場合も不正な値として 500 にする。`CHANGES.md` の「3 桁以外の場合は 500」の記載もあわせて更新する。

## 完了条件

- 1xx を書き込んだ後に別のステータスコードを書き込むと、そのステータスコードが送信され、アクセスログとメトリクスにも記録される。
- 1xx の後にハンドラーがエラーを返すと、アダプターがエラー応答を書く。
- 上記を確認するテストがある。
