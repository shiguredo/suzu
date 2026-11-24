# systemd のユニットが再起動のたびに標準出力と標準エラー出力のログを切り詰める

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-systemd-unit-log-truncation
- Polished: 2026-11-24

## 目的

Suzu がクラッシュして systemd が再起動しても、クラッシュの原因 (panic のスタックトレースなど) を残せるようにする。

## 現状

- `script/suzu.service` の `ExecStart` は、`/bin/bash -c '... 2>/var/log/suzu/stderr.log 1>/var/log/suzu/stdout.log'` でリダイレクトしている。`>` は起動のたびにファイルを空にする。
- `Restart=on-failure` と `RestartSec=60s` を設定しているため、panic で終了 (終了コード 2) すると 60 秒後に再起動し、そのときに panic のスタックトレースを含む stderr.log が消える。
  - goroutine の中の panic (recover されない) は Go のランタイムが標準エラー出力に出すため、zerolog のログには残らない。
- `LogsDirectory=suzu` で `/var/log/suzu` を作っているが、zerolog のログは `log_dir` に従って出力され、`log_dir` の既定の `.` では `WorkingDirectory` の `/home/sora/suzu` に出力される。ユニットとログの設定の意図が合っていない。
- `KillMode=control-group` は既定値と同じである。

## 設計方針

- `ExecStart` から bash とリダイレクトをやめ、`ExecStart=/home/sora/suzu/bin/suzu -C /home/sora/suzu/config.ini` にする。
- `StandardOutput=append:/var/log/suzu/stdout.log` と `StandardError=append:/var/log/suzu/stderr.log` を指定する (systemd 240 以降)。
- `LogsDirectory=suzu` は `/var/log/suzu` の所有を sora に保つために残す。
  - systemd は `StandardOutput=append:` と `StandardError=append:` のファイルを `LogsDirectory=` の作成より先に開くため、初回起動前に `/var/log/suzu` が存在しないと、サービスは 209/STDOUT (標準エラー出力の場合は 210/STDERR) で起動に失敗する。
  - `ExecStartPre` でディレクトリを作っても、`ExecStartPre` の標準出力も同じ設定で開かれるため間に合わない。
- `script/suzu.conf` に tmpfiles.d の設定 (`d /var/log/suzu 0755 sora sora -`) を追加し、`/etc/tmpfiles.d/suzu.conf` に配置して `systemd-tmpfiles --create` を実行する手順を `doc/USE.md` と `script/suzu.service` のコメントに書く。これにより初回起動前に `/var/log/suzu` を用意する。
- zerolog のログの出力先を `/var/log/suzu` にする場合の設定例を、`script/suzu.service` のコメントと `doc/USE.md` に書く。
  - `log_dir = /var/log/suzu` と `log_stdout = false` を例にし、既定の `log_name = suzu.jsonl` と組み合わせると `/var/log/suzu/suzu.jsonl` になることを書く。
  - `log_stdout = true` の場合は zerolog のログが標準出力に出力され、`StandardOutput=append:` の `stdout.log` に混ざることも書く。
- 既定値と同じ `KillMode=control-group` は削除する。

## 完了条件

- `script/suzu.service` の `ExecStart` が bash とリダイレクトを使わず、`StandardOutput=append:/var/log/suzu/stdout.log` と `StandardError=append:/var/log/suzu/stderr.log` を指定している。
- `script/suzu.conf` に `/var/log/suzu` を作る tmpfiles.d の設定 (`d /var/log/suzu 0755 sora sora -`) がある。
- `doc/USE.md` と `script/suzu.service` のコメントに、`/etc/tmpfiles.d/suzu.conf` の配置と `systemd-tmpfiles --create` の実行手順が書かれている。
- 初回起動時から、標準出力と標準エラー出力が `/var/log/suzu/stdout.log` と `/var/log/suzu/stderr.log` に追記される。
- 再起動しても、前回の起動までの標準出力と標準エラー出力のログが残る。
- ユニットのログの出力先と、zerolog のログの出力先の関係が `doc/USE.md` とユニットのコメントから分かる。
- `script/suzu.service` から既定値と同じ `KillMode=control-group` が削除されている。
