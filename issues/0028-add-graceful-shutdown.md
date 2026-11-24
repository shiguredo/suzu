# SIGTERM を受けたら処理中のセッションを待ってから停止する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/add-graceful-shutdown
- Polished: 2026-11-25

## 目的

デプロイや再起動のときに、処理中のセッションが終わるのを待ってから停止できるようにする。待つ時間には上限を設け、上限を過ぎたセッションは打ち切る。

## 現状

- `cmd/suzu/main.go` の `main` は、`context.Background()` から作った errgroup の ctx を `Server.Start` / `Server.StartExporter` に渡しており、シグナルを扱っていない。SIGTERM を受けると Go の既定の動作でプロセスが即座に終了する。
- `main` は `g.Wait()` がエラーを返すと `log.Fatal` で終了する。`context.Canceled` もエラーとして扱うため、シグナルで ctx を終了する設計にすると、Fatal レベルのログを出して終了コード 1 で終了する。
- `server.go` の `Start` / `StartExporter` は、ctx の終了時に defer で `Shutdown(ctx)` を呼ぶが、渡すのは終了済みの ctx である。`http.Server.Shutdown` は ctx が終了済みで処理中の接続がある場合、待たずに `context.Canceled` を返す。そのため、処理中の接続を待たず、`Shutdown` の error レベルのログを出す。
  - 現状、ctx が終了するのは、もう一方のサーバーが待ち受けに失敗した場合だけである。
- echo を使っていた頃から同じ挙動である。

## 設計方針

- `main` は `signal.NotifyContext` で SIGINT と SIGTERM を受けて ctx を終了する。`signal.NotifyContext` が返す stop 関数は defer で呼ぶ。`g.Wait` が `context.Canceled` を返した場合は、シグナルによる正常な停止として扱い、`log.Fatal` を呼ばず、終了コード 0 で終了する。
- `Start` / `StartExporter` は、終了済みの ctx ではなく、`context.WithoutCancel(ctx)` から `context.WithTimeout` で待つ時間の上限を付けた ctx で `Shutdown` を呼ぶ。
  - `Shutdown` が `context.DeadlineExceeded` を返した場合は `Close` で接続を閉じ、上限の値と Close したことを info レベルでログに出す。
  - それ以外の `Shutdown` のエラーと `Close` のエラーは error レベルでログに出す。
  - 音声ストリーミングのセッションは長時間続くため、上限を過ぎたセッションもこの経路で閉じる。
- 待つ時間の上限は `shutdown_timeout_sec` で設定できるようにする。`config.go` の `Config` に `ShutdownTimeoutSec` を追加し、未設定または 0 の場合は既定値の 30 秒を使う。負の値は `validateConfig` でエラーにする。`config_example.ini` に既定値と意味を記載し、systemd で動かす場合は `TimeoutStopSec` より短い値を指定する必要があることも書く。
- `CHANGES.md` の `## develop` に `[ADD]` のエントリとして、SIGTERM / SIGINT を受けたときの graceful shutdown の挙動と、設定項目と既定値を記載する。

## 完了条件

- SIGTERM または SIGINT を受けて ctx が終了すると、新しい接続を受け付けなくなり、処理中のセッションが終わるか上限の時間が過ぎてから終了する。
- 停止の際に、`Shutdown` が `context.Canceled` を返して error レベルのログを出さず、`main` も `context.Canceled` を `log.Fatal` で出さず、終了コード 0 で終了する。
- `shutdown_timeout_sec` が `config_example.ini` に記載され、既定値と上限を過ぎた場合の挙動が分かる。
- 上限を過ぎて `Close` した場合、そのことが info レベルでログに出る。
- `CHANGES.md` の `## develop` に `[ADD]` のエントリがある。
- `Start` / `StartExporter` の挙動を確認するテストがある。テストでは `Start` / `StartExporter` に渡す ctx を終了させ、処理中のリクエストがある状態で、リクエストが終わるまで待つことと、上限を過ぎた場合に `Close` されることを確認する。
- `main` の `context.Canceled` の扱いと終了コードは、コードレビューで確認する。
