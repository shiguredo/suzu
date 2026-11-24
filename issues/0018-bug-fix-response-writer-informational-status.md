# 1xx のステータスコードを送信済みのステータスコードとして記録する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-response-writer-informational-status
- Polished: 2026-11-24

## 目的

アクセスログの status とメトリクスの code が、1xx を書き込んだ場合も、クライアントが実際に受け取ったステータスコードになるようにする。

## 現状

- `middleware.go` の `responseWriter.WriteHeader` は、委譲した後に、渡されたステータスコードを送信済みとして記録する。コメントでは「1xx の情報レスポンスは扱わない (Suzu では送信しない)」としている。
- HTTP/1.1 では、net/http は 101 以外の 1xx を情報レスポンスとして送り、ヘッダーを送信済みにしない。その後の `WriteHeader` や `Write` で、最終的なステータスコードを送る。HTTP/2 でも 1xx は情報レスポンスとして送られ、ヘッダーを送信済みにしない。HTTP/2 では 101 も情報レスポンスとして扱うが、HTTP/2 では 101 を使わない。
- そのため 1xx を書き込むと、ラッパーは送信済みと判断し、その後の `WriteHeader` を無視する。アダプター (`errorHandlerFunc`) もエラー応答を書かない。
  - 1xx の後に `Write` や `Flush` をした場合や、何も書き込まずにハンドラーが終了した場合も、クライアントには 200 が送られるが、ラッパーは 1xx を送信済みとして記録するため、アクセスログとメトリクスは 1xx になる。
  - 1xx を書き込んだ後、最終的なステータスコードを送る前に通常の panic をした場合も、`serveWithRecover` は 500 を返さない。
  - 実測では、`WriteHeader(103)` の後にハンドラーがエラーを返すと、クライアントには 200 の空のボディが届き、アクセスログの status は 103 になった。
- `handler.go` の `createSpeechHandler` は、`SuzuError.Code` を 100 から 999 の範囲であれば `WriteHeader` に渡すため、`SuzuError.Code` が 1xx の場合にこの経路に入る。Amazon Transcribe の HTTP ステータスコードが 1xx になることは実質ない。

## 設計方針

- `responseWriter.WriteHeader` は、101 以外の 1xx を委譲するが、`status` と `headerSent` を更新しない。101 はこれまでどおり、委譲した後に `status` と `headerSent` を更新する (HTTP/2 では 101 を使わないため、101 を最終レスポンスとして扱う)。
- 1xx の後に `Write` や `Flush` をした場合は、net/http が 200 を送るため、既存の `markHeaderSent` が 200 を記録する。何も書き込まずにハンドラーが終了した場合は、`statusCode` が 200 を返す。
- 1xx を書き込んだ後、最終的なステータスコードを送る前に通常の panic をした場合は、`serveWithRecover` が `headerSent` を false と判断して 500 を返す。
- `responseWriter` のコメント「1xx の情報レスポンスは扱わない (Suzu では送信しない)」を、101 以外の 1xx は情報レスポンスとして送り、送信済みとして記録しない旨に更新する。
- `createSpeechHandler` は、`SuzuError.Code` が 200 未満の場合も不正な値として 500 にする (`createSpeechHandler` は HTTP/2 のみを受け付けるため、101 も最終ステータスコードとして扱わない)。
- `handler.go` のコメント「許容する範囲は 3 桁の整数とする（net/http の許容範囲）」も、許容する範囲を 200 から 999 とし、1xx は最終的なステータスコードにならない旨に更新する。
- `CHANGES.md` の「3 桁以外の場合は 500」の記載を「200 未満または 1000 以上の場合」に更新する。

## 完了条件

- 1xx を書き込んだ後に別のステータスコードを書き込むと、そのステータスコードが送信され、アクセスログとメトリクスにも記録される。
- 1xx の後に `Write` や `Flush` をした場合、または何も書き込まずにハンドラーが終了した場合も、クライアントが受け取る 200 がアクセスログとメトリクスに記録される。
- 1xx の後にハンドラーがエラーを返すと、アダプターがエラー応答を書き、アクセスログとメトリクスにもそのステータスコードが記録される。
- 1xx を書き込んだ後、最終的なステータスコードを送る前に通常の panic をした場合も、`serveWithRecover` が 500 を返し、アクセスログとメトリクスにも 500 が記録される。
- `SuzuError.Code` が 200 未満の場合に 500 を返す (既存の「connect error with invalid suzu error status code returns 500」のテストで確認する)。
- `httptest.ResponseRecorder` は 1xx を最終レスポンスとして扱うため、クライアントが受け取るステータスコードは実際の HTTP サーバーで確認するテストがある。
- 上記を確認するテストがある。
