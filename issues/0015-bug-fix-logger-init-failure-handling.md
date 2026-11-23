# ログの出力先の検査で panic する経路がある

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-logger-init-failure-handling
- Polished: 2026-11-23

## 目的

`log_dir` の設定に問題がある場合に、panic ではなく原因の分かるエラーで起動を中止する。

## 現状

- `logger.go` の `NewLogger` は、`if f, err := os.Stat(config.LogDir); os.IsNotExist(err) || !f.IsDir() { return nil, err }` で `log_dir` を検査している。
  - `os.Stat` が存在しない以外のエラー (権限がない、途中の要素がディレクトリでないなど) を返すと、`f` が nil のまま `f.IsDir()` を呼んで panic する。
  - `log_dir` がファイルの場合は、`err` が nil のため `(nil, nil)` を返す。`cmd/suzu/main.go` の `main` は `err` が nil なので続行し、`zlog.Logger = *logger` で nil を参照して panic する。
- `cmd/suzu/main.go` の `main` は、`NewLogger` が失敗した場合も `cannot parse config file` と出力しており、原因と文言が合っていない。
- ログの出力先のパスを `fmt.Sprintf("%s/%s", config.LogDir, logFilename)` で組み立てている。

## 設計方針

- `os.Stat` のエラーはすべてエラーとして返す。ディレクトリでない場合は、その旨のエラーを返す。ディレクトリが存在し `os.Stat` が成功する場合は、書き込めなくてもこの issue では検査しない。
- `main` は、ロガーの生成に失敗した場合に、ロガーの生成の失敗だと分かる文言で終了する。
- パスの組み立てには `filepath.Join` を使う。

## 完了条件

- `log_dir` が存在しない場合、ファイルである場合、`os.Stat` が権限エラーを返す場合のいずれも、panic せず、原因の分かるエラーで起動を中止する。
- 上記の各場合を確認するテストがある。
- ログファイルのパスを `filepath.Join` で組み立てている。
