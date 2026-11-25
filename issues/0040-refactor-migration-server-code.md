# net/http への移行で入ったサーバーのコードと echo の名残を整理する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/refactor-migration-server-code
- Polished: 2026-11-25

## 目的

echo から net/http への移行で入ったコードの重複と分かりにくい構成を整理し、echo の名残を取り除く。外部から見える挙動は変えない。

## 現状

- `server.go` の `Server.Start` と `Server.StartExporter` は、対象の `http.Server` と HTTPS かどうかの判定以外が同じ処理である。
- `middleware.go` に、ミドルウェアではないアダプターの `errorHandlerFunc` とエラー型の `httpError` が同居している。
  - `errorHandlerFunc` という名前は「エラーを処理するハンドラー」と読め、echo の `HTTPErrorHandler` と紛らわしい。実際は「エラーを返すハンドラー」である。
- `metrics.go` の `httpMetricsURLLabelValue` は、パターンを `[METHOD ][HOST]/[PATH]` の一般形として、タブや複数の空白まで扱って解釈している。ルートにマッチした場合のパターンは `server.go` の `handleRoute` が `method + " " + path` で作るものだけで、ルートにマッチしなかった場合 (404 や 405) は空である。
- `handler.go` の `soraHeader` は非公開の型だが、フィールドが `SoraChannelID` などの公開の名前になっている。echo の `BindHeaders` がリフレクションでフィールドに値を設定するために公開する必要があった名残で、`newSoraHeader` で取り出す今は不要である。`h.SoraChannelID` のように接頭辞も重複して見える。
- `server.go` の `appendCertsFromPEM` の引数名 `filepath` が、import している `path/filepath` パッケージを隠している。

## 設計方針

- `Start` と `StartExporter` の共通の処理を 1 つの関数にまとめる。
- `errorHandlerFunc` と `httpError` を別のファイル (`error_handler.go` など) に移し、`errorHandlerFunc` を「エラーを返すハンドラー」と分かる名前 (`errorReturningHandlerFunc` など) にする。`TestErrorHandlerFunc` も移動先のファイルに対応するテストファイルへ移す。
- `httpMetricsURLLabelValue` は、`handleRoute` が作る形式だけを `strings.Cut(pattern, " ")` で扱う。`pattern` が空の場合に `unknown` を返す処理は残す。
- `soraHeader` のフィールドを非公開の短い名前 (`channelID` など) にする。
- `appendCertsFromPEM` の引数名を `path` などに変える。
- 既存のテストが通ることを確認する。名前を変えるシンボルを参照しているテスト (`middleware_test.go` の `errorHandlerFunc`、`handler_test.go` の `soraHeader` のフィールド) は、参照とテスト名を新しい名前に合わせる。`httpMetricsURLLabelValue` の単体テストに一般形のケースがある場合は、`handleRoute` が作る形式のケースに絞る。いずれも外部から見える挙動の検証内容は変えない。

## 完了条件

- 上記の整理が終わり、既存のテストが通る。
- 外部から見える挙動 (応答、アクセスログ、メトリクス) が変わらない。
