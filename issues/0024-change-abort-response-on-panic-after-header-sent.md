# レスポンスヘッダーを送信した後に panic した場合はレスポンスを中断する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/change-abort-response-on-panic-after-header-sent
- Polished: {YYYY-MM-DD}

## 目的

レスポンスヘッダーを送信した後にハンドラーが panic した場合に、クライアントが途中で失敗したことを判別できるようにする。

## 現状

- `middleware.go` の `serveWithRecover` は、panic を recover して `PANIC-RECOVERED` を出力し、エラーをラッパーに記録する。ヘッダーを送信済みの場合はそのまま return し、レスポンスは正常に終了する。
- そのため、外部サービスへの接続に成功して結果を送っている途中で panic すると、クライアントには 200 の正常なレスポンスとして届き、結果が途中で切れたことを区別できない。
- echo の `middleware.Recover` も同じく、ヘッダーを送信済みの場合はレスポンスを正常に終了していた。そのため移行による後退ではない。

## 設計方針

- ヘッダーを送信済みの場合は、ログ、アクセスログ、メトリクスを記録した後に `panic(http.ErrAbortHandler)` で net/http にレスポンスを中断させる。HTTP/2 ではストリームのリセット、HTTP/1.1 では接続の切断になる。
- ログ、アクセスログ、メトリクスを記録してから中断するため、`newMainHandler` の記録の処理を、`serveWithRecover` の外側で `http.ErrAbortHandler` の再 panic より前に行う構成にする。
- `CHANGES.md` の `## develop` に、後方互換のない変更として記載する。

## 完了条件

- ヘッダーを送信した後に panic すると、クライアントはレスポンスの異常な終了 (HTTP/2 のストリームのリセット) を受け取る。
- その場合も `PANIC-RECOVERED`、アクセスログ、メトリクスが記録される。
- 上記を確認するテストがある。
