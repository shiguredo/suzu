# レスポンスヘッダーを送信した後に panic した場合はレスポンスを中断する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/change-abort-response-on-panic-after-header-sent
- Polished: 2026-11-24

## 目的

レスポンスヘッダーを送信した後にハンドラーが panic した場合に、クライアントが途中で失敗したことを判別できるようにする。

## 現状

- `middleware.go` の `serveWithRecover` は、`http.ErrAbortHandler` 以外の panic を recover して `PANIC-RECOVERED` を出力し、エラーをラッパーに記録する。ヘッダーを送信済みの場合はそのまま return し、レスポンスは正常に終了する。
- `http.ErrAbortHandler` の panic は、`serveWithRecover` が即座に再 panic し、`PANIC-RECOVERED`、アクセスログ、メトリクスを記録しない。net/http がスタックトレースを出力せずにレスポンスを中断する。
- そのため、外部サービスへの接続に成功して結果を送っている途中で `http.ErrAbortHandler` 以外の panic をすると、クライアントには 200 の正常なレスポンスとして届き、結果が途中で切れたことを区別できない。
- echo の `middleware.Recover` も同じく、ヘッダーを送信済みの場合はレスポンスを正常に終了していた。echo の `DefaultHTTPErrorHandler` は `c.Response().Committed` が true の場合に何もせずに return するためである。そのため移行による後退ではない。

## 設計方針

- ヘッダーを送信済みの場合、`http.ErrAbortHandler` 以外の panic では、ログ、アクセスログ、メトリクスを記録した後に `panic(http.ErrAbortHandler)` で net/http にレスポンスを中断させる。HTTP/2 ではストリームのリセット、HTTP/1.1 では接続の切断になる。
- ログ、アクセスログ、メトリクスを記録してから中断するため、`newMainHandler` の記録の処理を、`serveWithRecover` の外側で行い、`http.ErrAbortHandler` の再 panic より前に行う構成にする。具体的には、`serveWithRecover` は recover した後に中断が必要かを呼び出し元へ返し、`newMainHandler` がメトリクスとアクセスログを記録した後に `panic(http.ErrAbortHandler)` する。
- `http.ErrAbortHandler` の panic は、これまでどおり `serveWithRecover` で即座に再 panic し、ログ、アクセスログ、メトリクスを記録しない。この挙動は変えない。
- `serveWithRecover` と `newMainHandler` のコメントを、ヘッダー送信済みの panic ではレスポンスを中断する旨に更新する。
- `CHANGES.md` の `## develop` に、新しい `[CHANGE]` エントリとして、後方互換のない変更を記載する。

## 完了条件

- ヘッダーを送信した後に `http.ErrAbortHandler` 以外の panic をすると、クライアントはレスポンスの異常な終了 (HTTP/2 のストリームのリセット) を受け取る。
- その場合も `PANIC-RECOVERED`、アクセスログ、メトリクスが記録される。アクセスログの status とメトリクスの code は、送信済みの 200 のまま記録される。
- `http.ErrAbortHandler` を直接 panic した場合は、これまでどおり `PANIC-RECOVERED`、アクセスログ、メトリクスを記録しない。
- 上記を確認するテストがある。HTTP/2 のストリームのリセットは `httptest.ResponseRecorder` では確認できないため、`server_test.go` の `serveForTest` と `newH2CClientForTest` を使い、`newMainHandler` を HTTP/2 over TCP で待ち受けるテストで確認する。既存の `middleware_test.go` の「ヘッダー送信後に panic してもボディを追記せず、送信済みの 200 を記録する」は、`ServeHTTP` が `http.ErrAbortHandler` で再度 panic することを前提に更新する。
- `CHANGES.md` の `## develop` に、新しい `[CHANGE]` エントリがある。
