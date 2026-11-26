# エラー応答を echo と同じく {"message":"..."} の JSON 形式に戻す

- Created: 2026-11-26
- Completed: {YYYY-MM-DD}
- Branch: feature/change-error-response-json-format
- Polished: {YYYY-MM-DD}

## 目的

`/speech` などでクライアントに返すエラー応答を、echo を使っていた頃と同じ `{"message":"..."}` 形式の JSON に戻す。net/http への切り替えで text/plain の `Bad Request` などを返すようになり、Sora クライアントの `result.message` にエラーの文言が入らなくなる挙動の変化が出ているため。

## 現状

### クライアント側で起きた差

未登録のパス (`POST /nothing`) への応答は次のように変わった。

| | echo 時代 | 現在 (net/http) |
| --- | --- | --- |
| レスポンス | `{"message":"Not Found"}` | `404 page not found` |
| Sora クライアントへの Push | `result: {"message":"Not Found"}` | `result: 404` |
| アクセスログの `error` | `code=404, message=Not Found` | 出力されない |
| アクセスログの `bytes_out` | 0 | 19 |

### エラー応答の組み立て

ヘッダー未送信時のエラー応答は `middleware.go` の `errorHandlerFunc.ServeHTTP` が `http.Error` で組み立てている。panic 時の `serveWithRecover` も同じく `http.Error` を使う。404 と 405 は `http.ServeMux` が `http.NotFound` などで直接返すため、`errorHandlerFunc` を通らない。

echo の `DefaultHTTPErrorHandler` は、`*echo.HTTPError` 以外のエラーを `Code: 500` / `Message: StatusText(500)` にそろえ、`c.JSON` で `map[string]any{"message": ...}` を返していた。`Server` で `echo.Debug` を設定していないため、`debug = true` でも `message` だけが返っていた。echo の `JSON` は `application/json; charset=UTF-8` を付け、`nosniff` は付けていなかった。

## 設計方針

echo と同じ「ステータスコードの文言だけを JSON で返す」に戻す。原因のエラー文言はクライアントに返さず、従来どおりアクセスログの `error` にだけ残す。

- ヘッダー未送信のエラー応答は `Content-Type: application/json` で `{"message":"<StatusText>"}` を書き出す。ステータスコードは `httpError` の `code`、それ以外は 500 にする
- panic 時の応答も同じ JSON 形式にそろえる
- 404 / 405 も JSON 形式にそろえる。`http.ServeMux` が書いた本文を置き換える必要があるため、`newMainHandler` でレスポンスをバッファし、未送信のまま書き込まれた場合に JSON に差し替える
- `X-Content-Type-Options: nosniff` は付けない。echo と同じく、`http.Error` が付けていたヘッダーも含めてそろえる
- 404 / 405 でアクセスログの `error` を出すかは echo と同じにはしない。現行の「404 / 405 では error を出力しない」という設計を維持し、本 issue では扱わない
- `bytes_out` と `suzu_http_response_size_bytes` はボディの長さの分だけ増える。これは意図した変化とする

## 完了条件

`/speech` で発生するヘッダー未送信のエラー、panic、メインサーバーと Exporter の 404 / 405 の応答が、すべて `Content-Type: application/json` で `{"message":"..."}` を返すこと。Sora クライアントの `result.message` にステータスコードの文言が入ること。

## 解決方法

- `middleware.go` に JSON のエラー応答を書き出すヘルパーを追加し、`errorHandlerFunc.ServeHTTP` と `serveWithRecover` の `http.Error` を置き換える
- `newMainHandler` で `http.ServeMux` の前段にバッファ付きの `ResponseWriter` を置き、404 / 405 のような未送信で書かれたエラーを JSON に差し替える。`http.ServeMux` は 404 で `http.NotFound`、405 で `Method Not Allowed` と `Allow` を書くため、`Allow` はそのまま残す
- `middleware_test.go` の `TestErrorHandlerFunc` と `TestMainHandler`、`server_test.go` の 400 / 404 / 405 のアサーションを JSON 前提に変える
- `CHANGES.md` の `develop` で、エラー応答の形式を echo と同じ JSON に戻したことを `[CHANGE]` として記載する
- 影響を受ける利用者向けに、`Content-Type` が `text/plain` から `application/json` に戻ることと `bytes_out` の変化を書く
