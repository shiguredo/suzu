# SIGTERM を受けたら処理中のセッションを待ってから停止する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/add-graceful-shutdown
- Polished: {YYYY-MM-DD}

## 目的

デプロイや再起動のときに、処理中のセッションを途中で切らずに停止できるようにする。

## 現状

- `cmd/suzu/main.go` の `main` は、`context.Background()` から作った errgroup の ctx を `Server.Start` / `Server.StartExporter` に渡しており、シグナルを扱っていない。SIGTERM を受けると Go の既定の動作でプロセスが即座に終了する。
- `server.go` の `Start` / `StartExporter` は、ctx の終了時に defer で `Shutdown(ctx)` を呼ぶが、渡すのは終了済みの ctx である。`http.Server.Shutdown` は ctx が終了済みだと待たずに `context.Canceled` を返すため、処理中の接続を待たず、毎回 error レベルのログを出す。
  - 現状、ctx が終了するのは、もう一方のサーバーが待ち受けに失敗した場合だけである。
- echo を使っていた頃から同じ挙動である。

## 設計方針

- `main` は `signal.NotifyContext` で SIGINT と SIGTERM を受けて ctx を終了する。
- `Start` / `StartExporter` は、終了済みの ctx ではなく、`context.WithoutCancel` に待つ時間の上限を付けた ctx で `Shutdown` を呼ぶ。上限を過ぎたら `Close` で接続を閉じる。
- 待つ時間の上限は設定で指定できるようにし、`config_example.ini` に記載する。
- 音声ストリーミングのセッションは長時間続くため、上限を過ぎたセッションの扱い (閉じたことをログに出すなど) を決める。
- `CHANGES.md` の `## develop` に `[ADD]` として記載する。

## 完了条件

- SIGTERM を受けると新しい接続を受け付けなくなり、処理中のセッションが終わるか上限の時間が過ぎてから終了する。
- 停止の際に、`Shutdown` が `context.Canceled` を返して error レベルのログを出すことがない。
- 上記を確認するテストがある。
