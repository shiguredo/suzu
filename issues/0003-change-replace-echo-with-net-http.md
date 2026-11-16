# echo をやめて net/http に切り替える

- Created: 2026-11-16
- Completed: {YYYY-MM-DD}
- Branch: feature/change-replace-echo-with-net-http
- Polished: 2026-11-16

## 目的

HTTP サーバーの実装を `github.com/labstack/echo/v4` と `github.com/labstack/echo-contrib` から標準ライブラリの `net/http` に切り替え、echo への依存をなくす。

- echo v4 は README で「バグ修正は 2026-12-31 まで、セキュリティ修正は 2027-12-31 まで、新規プロジェクトは v5 を使うこと」と明記している。使い続けるには import パスから変わる v5 (`github.com/labstack/echo/v5`) への移行が必要になる。
- Prometheus 用に使っている `echo-contrib/prometheus` パッケージは Deprecated になっている。
- Suzu が echo から使っている機能は「現状」に挙げた範囲に限られ、いずれも `net/http` と `prometheus/client_golang` で置き換えられる。メインサーバーはすでに `http.Server` を自前で組み立てて `ListenAndServe` / `ListenAndServeTLS` を直接呼んでおり、echo はルーターとミドルウェアの入れ物としてしか使っていない。
- echo を外せば、フレームワークのメジャーバージョン移行が不要になり、依存ライブラリ更新の対象も減る。

## 現状

### echo の利用箇所

- `server.go`
  - `Server` 構造体は `echo` / `echoExporter` の 2 つの `*echo.Echo` を持つ。
  - `NewServer` はメインサーバー用に `echo.New()` を作り、その `Server` フィールドを自前の `http.Server` に差し替えている。この `http.Server` には `Protocols` (HTTP/1、HTTP/2、HTTP/2 over TCP)、`HTTP2`、`IdleTimeout`、クライアント認証用の `TLSConfig` を設定している。
  - ミドルウェアは `middleware.RemoveTrailingSlash` (Pre)、`middleware.RequestLoggerWithConfig`、`middleware.Recover`、`prometheus.NewPrometheus("echo", nil)` の `HandlerFunc` の順に登録している。
  - ルートは `GET /.ok`、`POST /speech`、`POST /test`、`POST /dump` の 4 本。
  - Exporter は別の `echo.New()` に `SetMetricsPath` で `GET /metrics` を登録している。
  - `Start` は `s.echo.Server` の `ListenAndServe` / `ListenAndServeTLS` を直接呼び、終了時に `s.echo.Shutdown` を呼ぶ。
  - `StartExporter` は `s.echoExporter` の `Start` / `StartTLS` を呼び、終了時に `s.echoExporter.Shutdown` を呼ぶ。
- `handler.go` の `createSpeechHandler`
  - `echo.HandlerFunc` を返す。`echo.Context` から使っているのは `Request()`、`Response()` の `Header()` / `Write()` / `Flush()`、`NoContent()` だけ。
  - リクエストヘッダーは `echo.DefaultBinder` の `BindHeaders` で、`soraHeader` の `header` タグに従って取り出している。フィールドはすべて string。
  - エラーは `echo.NewHTTPError` または素の `error` として返し、echo の `DefaultHTTPErrorHandler` がレスポンスに変換する。レスポンスが送信済みと echo が判断した場合は何もしない。
- `healthcheck_handler.go` の `healthcheckHandler` は `c.JSON` でバージョンを返す。
- `healthcheck_handler_test.go` と `test_handler_test.go` は `echo.New()` と `e.NewContext` で `echo.Context` を作ってハンドラーを直接呼んでいる。`test_handler_test.go` は、返ってきたエラーを `*echo.HTTPError` に型アサーションしてステータスコードを検証している。
- `go.mod` の直接依存に `github.com/labstack/echo/v4` と `github.com/labstack/echo-contrib` がある。`github.com/prometheus/client_golang` は echo-contrib 経由の間接依存になっている。

### 現在の外部挙動

サーバーを実際に起動して確認した結果は次の通り。

| リクエスト | 応答 |
| --- | --- |
| `GET /.ok` | 200、`Content-Type: application/json`、バージョンを含む JSON |
| HTTP/1.1 での `POST /speech` | 400、`{"message":"Bad Request"}` |
| `sora-audio-streaming-language-code` ヘッダーなしの `POST /test` (HTTP/2) | 500、`{"message":"Internal Server Error"}` |
| `GET /speech` | 405、`Allow: OPTIONS, POST`、`{"message":"Method Not Allowed"}` |
| `OPTIONS /speech` | 204、`Allow: OPTIONS, POST` |
| 登録していないパス | 404、`{"message":"Not Found"}` |
| `POST /test/` (末尾スラッシュ付き) | `POST /test` と同じ扱い |
| `GET //test` | 404 |
| Exporter の `GET /metrics` | `echo_requests_total` / `echo_request_duration_seconds` / `echo_request_size_bytes` / `echo_response_size_bytes` (ラベルは `code` / `method` / `host` / `url`) と `go_*` / `process_*` / `promhttp_metric_handler_*` |

- アクセスログは zerolog の info で出力している。キーは `remote_ip` / `host` / `user_agent` / `uri` / `status` / `error` / `latency` / `bytes_in` / `bytes_out`、メッセージは HTTP メソッド。パスが `/.ok` で始まるリクエストは出力しない。
- アクセスログの `error` は、ハンドラーまたは echo のルーターがエラーを返した場合に出力される。`echo.HTTPError` の場合は `code=404, message=Not Found` の形式になる。ルーターが返す 404 / 405 もこの形式で出力される。
- アクセスログの `bytes_out` は、ハンドラーが返したエラーを echo のエラーハンドラーが応答に変換した場合、0 のままになる。
- メトリクスの `url` ラベルには、ルートにマッチしなかった場合リクエストパスがそのまま入る。メソッドだけが合わない 405 の場合はルートのパスが入る。`host` ラベルには `Host` ヘッダーの値が入る。どちらもクライアントが自由に指定でき、時系列の数に上限がない。
- panic 時は `middleware.Recover` が echo 内部のロガー (gommon) にスタックトレースを出力する。zerolog を経由しないため `log_dir` などのログ設定に従わない。

## 設計方針

### 基本

- ルーティングは `http.ServeMux` のメソッド付きパターン (`GET /.ok`、`POST /speech` など) を使う。外部のルーターライブラリは追加しない。
- 登録していないパスの 404、メソッドが合わない場合の 405、正規化されていないパスへのリダイレクトは `http.ServeMux` が直接応答し、後述のアダプターを通らない。
- `/speech` / `/test` / `/dump` の処理内容は変えない。HTTP/2 必須の判定、レスポンスヘッダーを外部サービス接続成功後に送るタイミング、結果の逐次送信、リトライ、各エラー時のステータスコードを維持する。
- それ以外の外部挙動は echo との互換を維持せず、`net/http` の標準動作に寄せる。変わる点は「後方互換のない変更」に列挙する。

### ハンドラー

- ハンドラーは `func(http.ResponseWriter, *http.Request) error` 型とし、これを `http.Handler` に変換するアダプターを 1 箇所に置く。理由は次の通り。
  - `createSpeechHandler` はエラーを返す経路が多い。「ヘッダー未送信ならエラー応答を書き、送信済みなら何もしない」という判定を各経路に書かず 1 箇所に集約する。
  - アクセスログの `error` に、ハンドラーが返したエラーを出す現行の挙動を保てる。
  - 既存テストはハンドラーが返すエラーを検証しており、検証内容を変えずに済む。
- アダプターは、ヘッダー未送信の場合に限り、返ってきたエラーを `http.Error` で応答に変換する。ステータスコードを持つエラー型 (`echo.HTTPError` の置き換え) ならそのコード、それ以外は 500 とする。ボディは `http.StatusText` の文言だけとし、内部エラーの文言は含めない。ヘッダーを送信済みかどうかは、「ミドルウェア」で述べる ResponseWriter のラッパーが持つ状態で判定する。
- アダプターは、返ってきたエラーをラッパーに記録し、アクセスログから参照できるようにする。
- `echo.Context` の呼び出しは次のように置き換える。
  - `c.Response().Flush()` は `http.NewResponseController` の `Flush` にする。
  - `c.NoContent(code)` は `WriteHeader(code)` にする。
  - `BindHeaders` は `r.Header.Get` にする。`soraHeader` の `header` タグは不要になるため削除する。`r.Header.Get` は失敗しないため `INVALID-HEADER` の分岐も削除する。
  - `c.JSON` は `encoding/json` での書き出しにする。`/.ok` のステータスコード、`Content-Type`、ボディは変えない。
- `handler.go` にある echo の streaming-response cookbook への参照コメントは削除する。

### ミドルウェア

`middleware.RequestLoggerWithConfig` と `middleware.Recover` は自前の実装に置き換える。`middleware.RemoveTrailingSlash` は置き換えず、末尾スラッシュ付きのパスは受け付けなくなる。Prometheus 用のミドルウェアは「メトリクス」で扱う。

メインサーバーのハンドラーは、外側から次の順に重ねる。

1. ResponseWriter のラッパーの生成とアクセスログ
2. メトリクス
3. recover
4. `http.ServeMux`
5. アダプター
6. ハンドラー

- ResponseWriter のラッパーは最も外側で 1 回だけ生成し、内側のすべての層に同じものを渡す。ラッパーは、実際に送信したステータスコード、書き込みバイト数、ヘッダーを送信済みかどうか、アダプターと recover が記録するエラーを保持する。
  - ヘッダーの送信は `WriteHeader` だけでなく、最初の `Write` と `Flush` でも起きる。`createSpeechHandler` は外部サービス接続成功後に `Flush` でヘッダーを送るため、`WriteHeader` だけを見るラッパーでは、その後に呼ばれた `WriteHeader` のコードを誤って記録する (HTTP/2 では送信済み後の `WriteHeader` はログも出さず無視されることを確認済み)。`Flush` と `Write` も送信済みとして扱い、送信済み後の `WriteHeader` はラッパーで無視する。
  - `Flush` を送信済みとして扱うため、ラッパー自身が `FlushError` を実装する。`http.ResponseController` の `Flush` は `FlushError`、`http.Flusher`、`Unwrap` の順に探すため、`Unwrap` だけを実装したラッパーでは `Flush` がラッパーを素通りし、送信済みを記録できない。`FlushError` では送信済みを記録してから、元の ResponseWriter に対する `http.NewResponseController` の `Flush` に委譲する。
  - `Unwrap` は、`Flush` 以外の `http.ResponseController` の操作が元の ResponseWriter に届くように実装する。
  - この構成で、HTTP/2 over TCP のレスポンスが逐次送信されること、`Flush` の後に `WriteHeader` で別のコードを指定しても記録が 200 のままであること、`Flush` の後にハンドラーがエラーを返してもボディが追記されないことを確認済み。
- アクセスログは、現行と同じキー、メッセージ、除外条件で zerolog に出力する。
  - `remote_ip` は `RemoteAddr` のホスト部とする (echo v4.16.3 の既定と同じで、`X-Forwarded-For` などは参照しない)。
  - `error` は、ラッパーにエラーが記録されている場合だけ出力する。`http.ServeMux` が直接応答する 404 / 405 / リダイレクトではエラーが記録されないため、出力しない。
- recover は panic を捕まえて zerolog にスタックトレース付きの error として出力し、panic の内容をエラーとしてラッパーに記録する。ヘッダー未送信なら、アダプターと同じく `http.StatusText` の文言をボディにした 500 を返す。`http.ErrAbortHandler` は再度 panic させる。recover しない場合、HTTP/2 ではストリームがリセットされ、クライアントはステータスコードを受け取れない (確認済み)。
- recover をメトリクスより内側に置くことで、panic で 500 を返した場合もアクセスログとメトリクスに反映される。

### メトリクス

`echo-contrib/prometheus` の代わりに `prometheus/client_golang` で次の 4 種を定義する。

| メトリクス | 種別 | バケット |
| --- | --- | --- |
| `suzu_http_requests_total` | counter | - |
| `suzu_http_request_duration_seconds` | histogram | `prometheus.DefBuckets` (現行と同じ) |
| `suzu_http_request_size_bytes` | histogram | 1 KB から 10 MB (現行と同じ) |
| `suzu_http_response_size_bytes` | histogram | 1 KB から 10 MB (現行と同じ) |

- ラベルは `code` / `method` / `url` の 3 つとする。
  - `code` は実際に送信したステータスコード。
  - `method` は HTTP メソッド。`net/http` が定数として定義しているメソッド以外は `unknown` にまとめる。
  - `url` はマッチしたルートのパス (`/speech` など)。`http.ServeMux` が `r.Pattern` に設定するパターン (`POST /speech` など) からメソッドを除いたものを使う。`r.Pattern` が空の場合は `unknown` とし、リクエストパスをそのまま入れない。たとえば 404、405、および正規化後のパスとメソッドがどのルートにも一致しないリダイレクト (`GET //speech` など) では、`r.Pattern` は空になる。正規化後のパスとメソッドがルートに一致するリダイレクト (`POST //speech` など) では、`r.Pattern` にそのルートのパターンが入るため、`url` はそのルートのパスになる。`r.Pattern` は `ServeMux` の `ServeHTTP` に渡したリクエストに設定されるため、メトリクスの層から `ServeMux` までは同じ `*http.Request` を渡す。
  - `host` ラベルは引き継がない。`Host` ヘッダーはクライアントが自由に指定でき、時系列の数に上限がなくなる。インスタンスの識別は Prometheus が付ける `instance` ラベルでできる。
- サイズのバケットの境界は 1 KB / 2 KB / 5 KB / 10 KB / 100 KB / 500 KB / 1 MB / 2.5 MB / 5 MB / 10 MB とする (1 KB は 1024 バイト)。現行の echo-contrib と同じ値である。
- リクエストサイズは現行と同じ概算 (パス、メソッド、プロトコル、ヘッダー、`Host`、`Content-Length` の合計) とする。
- 計測対象は現行と同じく、メインサーバーの全リクエスト (`/.ok` を含む)。
- コレクターはパッケージで 1 回だけ `prometheus.DefaultRegisterer` に登録する。`NewServer` はテストで複数回呼ばれるため、`NewServer` の中で登録すると 2 回目以降が重複登録になる。

### Exporter

- 専用の `http.Server` と `http.ServeMux` を用意し、`GET /metrics` に `promhttp.Handler()` を登録する。
- `exporter_https` が有効な場合は `tls_fullchain_file` / `tls_privkey_file` で `ListenAndServeTLS` を呼ぶ。現行と同じく、クライアント認証、アクセスログ、メトリクス計測は行わない。

### サーバーの起動と停止

- `Server` 構造体は 2 つの `*http.Server` を持つ形にする。
- `Start` / `StartExporter` のシグネチャと、`ctx` の終了時に `Shutdown` を呼んで `ctx.Err()` を返す挙動は変えない。`cmd/suzu/main.go` の変更は不要。
- メインサーバーの `http.Server` の設定 (`Protocols`、`HTTP2`、`IdleTimeout`、`TLSConfig`) は現行のままとする。

### 依存ライブラリ

- `go.mod` から `github.com/labstack/echo/v4` と `github.com/labstack/echo-contrib` を削除し、`github.com/prometheus/client_golang` を直接依存にする。間接依存は `go mod tidy` の結果に従う。

### テスト

- `healthcheck_handler_test.go` と `test_handler_test.go` は、`echo.New()` と `e.NewContext` をやめ、`httptest.NewRequest` と `httptest.NewRecorder` をハンドラーに直接渡す形に書き換える。検証している内容 (ステータスコード、ボディ、返却エラー、ログ) は変えない。`*echo.HTTPError` の検証は置き換えたエラー型で行う。
- 既存テストはハンドラーを直接呼ぶため、ルーティングとミドルウェアを通らない。切り替えで壊れやすいのはこの部分のため、`Server` のハンドラー全体を HTTP/2 over TCP で実際に待ち受け、標準ライブラリの HTTP クライアントで次を確認するテストを追加する。
  - `POST /test` の結果を逐次受信できる。
  - HTTP/1.1 での `POST /speech` が 400 になる。
  - 登録していないパスが 404、`GET /speech` が 405 になる。
  - Exporter の `/metrics` から `suzu_http_requests_total` を取得できる。
- ResponseWriter のラッパーについて、`Flush` の後に `WriteHeader` を呼んでも記録したステータスコードが変わらず、送信済みと判定されることを確認するテストを追加する。
- モックやスタブは使わない。

### ドキュメント

- `0001-doc-prometheus-exporter.md` が本 issue より先に完了していた場合、`doc/USE.md` には `echo_*` のメトリクス一覧が記載されている。その場合は `doc/USE.md` のメトリクス名とラベルを本 issue で更新する。先に完了していなければ、本 issue でのドキュメントの変更はない。

## 後方互換のない変更

`CHANGES.md` の `## develop` に `[CHANGE]` として記載する。

- Prometheus の HTTP メトリクス
  - 名前が `echo_*` から `suzu_http_*` に変わる。
  - `host` ラベルがなくなる。
  - `url` ラベルは、ルートにマッチしなかった場合に `unknown` になる。メソッドだけが合わない 405 も `unknown` になる (現行はルートのパスが入る)。
  - `method` ラベルは、`net/http` が定数として定義しているメソッド以外が `unknown` になる。
  - `code` ラベルは実際に送信したステータスコードになる。
- エラー応答のボディ
  - `{"message":"Bad Request"}` 形式の JSON から `text/plain` に変わる。400 / 404 / 405 / 500 が対象。
  - 400 / 500 のボディは `http.StatusText` の文言 (`Bad Request` / `Internal Server Error`) になる。
  - 404 / 405 のボディは `http.ServeMux` が返す文言 (`404 page not found` / `Method Not Allowed`) になる。
  - `createSpeechHandler` が `echo.NewHTTPError(http.StatusInternalServerError, err)` を返す経路は、ヘッダー未送信の場合に内部エラーの文言をボディに含めていた。これを含めなくなる。
- 末尾スラッシュ付きのパス (`/speech/` など) は 404 になる。Sora の `audio_streaming_url` を末尾スラッシュ付きで設定している利用者は設定の変更が必要になる。
- ルーティングの細部が `http.ServeMux` の標準動作になる。
  - `OPTIONS` は 204 ではなく 405 になる。
  - 405 の `Allow` ヘッダーは `OPTIONS` を含まなくなる。
  - `GET /.ok` は `HEAD` にも応答する。
  - `//speech` のような正規化されていないパスは 404 ではなく 307 リダイレクトになる。
- アクセスログ
  - キーは変わらない。
  - `status` と `bytes_out` は実際に送信した値になる。
  - `error` の文言は echo の `code=400, message=Bad Request` 形式ではなくなる。
  - `http.ServeMux` が直接応答する 404 / 405 では `error` を出力しなくなる (現行は `code=404, message=Not Found` のように出力している)。
- panic 時のスタックトレースは zerolog 経由で出力されるようになる。

## 完了条件

- Go のソースコードと `go.mod` / `go.sum` に `github.com/labstack` が残っていない。
- `make test` と、CI で実行している staticcheck が通る。
- `/speech` / `/test` / `/dump` の正常系と、各エラー時のステータスコードが切り替え前と同じである。
- `Server` のハンドラー全体を通したテストで、HTTP/2 over TCP でのレスポンスの逐次受信を確認できる。
- Exporter の `/metrics` から `suzu_http_*` の 4 種を取得でき、`echo_*` が出力されない。
- `CHANGES.md` に「後方互換のない変更」の内容が `[CHANGE]` として記載されている。
- `doc/USE.md` に `echo_*` のメトリクスが記載されている場合は、`suzu_http_*` とそのラベルに更新されている。

## 関連 issue

- `0001-doc-prometheus-exporter.md` と `0002-add-prometheus-metrics.md` は、現状の記述が `echo-contrib/prometheus` と `echo_*` を前提にしている。本 issue の完了後に同期が必要になる。
