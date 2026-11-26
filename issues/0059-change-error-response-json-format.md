# エラー応答を echo と同じく {"message":"..."} の JSON 形式に戻す

- Created: 2026-11-26
- Completed: {YYYY-MM-DD}
- Branch: feature/change-error-response-json-format
- Polished: 2026-11-26

## 目的

`/speech` などでクライアントに返すエラー応答を、echo を使っていた頃と同じ `{"message":"..."}` 形式の JSON に戻す。net/http への切り替えで text/plain の `Bad Request` などを返すようになり、Sora クライアントの `result.message` にエラーの文言が入らなくなる挙動の変化が出ているため。

## 現状

### クライアント側で起きた差

未登録のパス (`POST /nothing`) への応答は、Sora クライアントから見ると次のように変わった。

| | echo 時代 | 現在 (net/http) |
| --- | --- | --- |
| レスポンス | `{"message":"Not Found"}` | `404 page not found` |
| Sora クライアントへの Push | `result: {"message":"Not Found"}` | `result: 404` |
| アクセスログの `error` | `code=404, message=Not Found` | 出力されない |
| アクセスログの `bytes_out` | 24 | 19 |

- Sora クライアントへの Push の列は、Sora クライアントを使った実測による。リポジトリ内に一次資料はない
- `bytes_out` の 24 は、echo の `DefaultJSONSerializer.Serialize` が `json.NewEncoder` の `Encode` で末尾に改行を付けるため、`{"message":"Not Found"}` の 23 バイトに 1 バイトが加わった値
- `bytes_out` の 19 は、`http.NotFound` が書く `404 page not found\n` の長さ

### エラー応答の組み立て

ヘッダー未送信時のエラー応答は `middleware.go` の `errorHandlerFunc.ServeHTTP` が `http.Error` で組み立てている。panic 時の `serveWithRecover` も同じく `http.Error` を使う。

404 と 405 は `http.ServeMux` が直接返すため `errorHandlerFunc` を通らない。どちらも `http.Error` で本文を書き、`text/plain; charset=utf-8` と `nosniff` を付ける。

| | 本文 | 長さ |
| --- | --- | --- |
| 404 | `404 page not found` | 19 |
| 405 | `Method Not Allowed` | 19 |

405 は `Allow` ヘッダーを設定したうえで `http.Error` を呼ぶ。`http.Error` は `Content-Length` だけを削除するため、`Allow` は残る。

Exporter は `Server` の `exporterServer` に `Handler: exporterMux` を設定しており、`newMainHandler` を通らないためメインサーバーと扱いが異なる。

echo の `DefaultHTTPErrorHandler` は、`*echo.HTTPError` 以外のエラーを `Code: 500` / `Message: StatusText(500)` にそろえ、`c.JSON` で `map[string]any{"message": ...}` を返していた。`Server` で `echo.Debug` を設定していないため、`debug = true` でも `message` だけが返っていた。echo の `JSON` が付ける `Content-Type` は `application/json` で、`charset` は付かない。`nosniff` も付けていなかった。

## 設計方針

echo と同じ「ステータスコードの文言だけを JSON で返す」に戻す。原因のエラー文言はクライアントに返さず、従来どおりアクセスログの `error` にだけ残す。

- ヘッダー未送信のエラー応答は `Content-Type: application/json` で `{"message":"<StatusText>"}` と末尾の改行を書き出す。ステータスコードは `httpError` の `code`、それ以外は 500 にする
- 本文の書き出しは 1 箇所のヘルパーに集約し、`http.Error` が行っていた `Content-Length` の削除も行う。`Content-Length` を残すと、長さが変わる本文の書き込みで `net/http` が `ErrContentLength` を返す
- panic 時の応答も同じ JSON 形式にそろえる
- 404 / 405 も JSON 形式にそろえる。`http.ServeMux` は応答を書き換えられる形で返さないため、レスポンスをバッファするのではなく、ルーティングを `http.ServeMux` に任せたうえで 404 / 405 のときに限り JSON を書く。メインサーバーと Exporter の両方に同じ適用をする
- `X-Content-Type-Options: nosniff` は付けない。echo と同じく、`http.Error` が付けていた `nosniff` も含めてそろえる
- 404 / 405 でアクセスログの `error` を出すかは echo と同じにはしない。現行の「404 / 405 では error を出力しない」という設計を維持し、本 issue では扱わない
- ステータスコードに既定型文の無いコード (`http.StatusText(code)` が空文字を返すコード) をハンドラーが返した場合は、`message` を空文字にする。この経路は `httpError` を作る箇所が決まったコードしか使っていないため、実質的には発生しない
- `bytes_out` と `suzu_http_response_size_bytes` は、本文の長さが変わった分だけ変化する。0 から増えるのではなく、既に書いていた本文が JSON に置き換わるだけなので、意図した変化とする

### 404 / 405 をバッファで差し替えない理由

`newMainHandler` の `rw` は `errorHandlerFunc` とアクセスログ・メトリクスが同じ値を見ることを前提にしている。`mux` の前段にバッファ付きの `ResponseWriter` を挟むと、次の問題が起きる。

- `errorHandlerFunc.ServeHTTP` の `w.(*responseWriter)` が失敗し、別のラッパーが生成されて `rw.err` がアクセスログに届かない
- `createSpeechHandler` は `flushResponse` で `http.NewResponseController(w).Flush()` を呼んで結果を逐次送信する。バッファ型が `FlushError` を実装すると Flush の時点で 200 が確定して本文が欠け、実装しなければ終了時の書き込みで `http: superfluous response.WriteHeader call` になる
- 正常系の本文もすべてバッファ経由になり、`/speech` のストリーミングの前提が崩れる

## 完了条件

次のすべてが `Content-Type: application/json` で `{"message":"..."}` を返すこと。

- `/speech` で発生するヘッダー未送信のエラー (400 / 500)
- panic 時の 500
- メインサーバーの 404 / 405
- Exporter の 404 / 405

Sora クライアントから見て、`result.message` にステータスコードの文言が入ること。`/speech` の結果が逐次送信されることと、アクセスログの `error` が従来どおり出力されることが変わらないこと。

## 解決方法

- `middleware.go` に JSON のエラー応答を書き出すヘルパーを追加し、`errorHandlerFunc.ServeHTTP` と `serveWithRecover` の `http.Error` を置き換える
- 404 / 405 の JSON 化は、`newMainHandler` のバッファではなく、`http.ServeMux` の応答を 404 / 405 に限って JSON で置き換える方法にする。`mux.Handler` はマッチしない場合に `http.NotFoundHandler` や 405 用のハンドラーを返し、パターンが空の文字列で返るため判定に使える
  - ただし正規化されていないパスの 307 リダイレクトもパターンが空で返る場合がある。返されたハンドラーがリダイレクト用かどうか、またはステータスコードで 3xx を除外して、404 / 405 だけを対象にすること
  - `Allow` ヘッダーの付与は `http.ServeMux` の挙動に合わせて同じ値を書く。メソッドを名前の順に並べ、GET のルートがある場合は HEAD も含める (`server.go` の `handleRoute` が組み立てる `allowHeader` と同じ値)
- Exporter の 404 / 405 にも同じ JSON 化を適用する。Exporter は `newMainHandler` を通らないため、`NewServer` の `exporterMux` と `Handler` の組み立てを対象に含める。Exporter はアクセスログとメトリクスを行わないため、応答の形式だけをそろえる
- `middleware_test.go` の `TestErrorHandlerFunc` と `TestMainHandler` のアサーションを JSON 前提に変える。レスポンスの形式を見ていないサブテスト (`Flush` 後にエラーを返す経路、`http.ErrAbortHandler` の経路) は対象外。404 のサブテストはアクセスログの `error` に関する検証なので、コメントだけを現状に合わせる
- `server_test.go` の 400 のアサーションを JSON 前提に変える。404 / 405 の本文と `Content-Type` は現状検証されていないため、新たに検証を追加する
  - メインサーバーと Exporter の両方で、404 / 405 の本文、`Content-Type`、`Allow` の値を検証する
  - `TestServerWithTrailingSlashRemovalAndOptionsResponseDisabled` は Exporter とメインの両方の 405 の `Allow` を検証しているため、JSON 化後も値が変わらないことを確認する
- `test_handler_test.go` の `TestSpeechHandler` は `newMainHandler` を通さずハンドラーを直接呼ぶため、変更の影響を受けないことを確認する
- `CHANGES.md` の `develop` を更新する。echo から net/http への切り替えはまだリリースされていないため、新しい `[CHANGE]` を別途積まず、既存の「エラー応答を変更する」の項目を最終的な応答形式に合わせて書き換える。同じ develop に矛盾する説明が並ぶのを防ぐ
  - 「ボディを {"message":"..."} 形式の JSON から text/plain に変更する」とその子項目 (400 / 500 と 404 / 405 のボディの記述) を、echo と同じ JSON のままとする記述に直す
  - 「X-Content-Type-Options: nosniff ヘッダーを付ける」の行を削除する。`nosniff` を付けない方針のため
  - 変更履歴は派生元ブランチとの最終的な差分だけを書く規約のため、中間状態 (text/plain) を経たことは書かない
- 影響を受ける利用者向けに、`Content-Type` が `application/json` になり、`bytes_out` の値が変わることを書く
