# systemd のユニットが再起動のたびに標準出力と標準エラー出力のログを切り詰める

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-systemd-unit-log-truncation
- Polished: {YYYY-MM-DD}

## 目的

Suzu がクラッシュして systemd が再起動しても、クラッシュの原因 (panic のスタックトレースなど) を残せるようにする。

## 現状

- `script/suzu.service` の `ExecStart` は、`/bin/bash -c '... 2>/var/log/suzu/stderr.log 1>/var/log/suzu/stdout.log'` でリダイレクトしている。`>` は起動のたびにファイルを空にする。
- `Restart=on-failure` と `RestartSec=60s` を設定しているため、panic で終了 (終了コード 2) すると 60 秒後に再起動し、そのときに panic のスタックトレースを含む stderr.log が消える。
  - goroutine の中の panic (recover されない) は Go のランタイムが標準エラー出力に出すため、zerolog のログには残らない。
- `LogsDirectory=suzu` で `/var/log/suzu` を作っているが、zerolog のログは `log_dir` に従って出力され、`log_dir` の既定の `.` では `WorkingDirectory` の `/home/sora/suzu` に出力される。ユニットとログの設定の意図が合っていない。
- `KillMode=control-group` は既定値と同じである。

## 設計方針

- `ExecStart` から bash とリダイレクトをやめ、`StandardOutput=append:` と `StandardError=append:` で追記する (systemd 240 以降)。
- zerolog のログの出力先を `/var/log/suzu` にする場合の設定例を、ユニットのコメントかドキュメントに書く。
- 既定値と同じ `KillMode=control-group` は削除する。

## 完了条件

- 再起動しても、前回の起動までの標準出力と標準エラー出力のログが残る。
- ユニットのログの出力先と、zerolog のログの出力先の関係がドキュメントから分かる。
