# 末尾スラッシュの除去がパーセントエンコードされたスラッシュ (%2F) も取り除く

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-trailing-slash-removal-encoded-slash
- Polished: 2026-11-24

## 目的

末尾スラッシュの除去 (`disable_trailing_slash_removal` が false の場合の既定の挙動) が、エスケープされていないリテラルのスラッシュだけを対象にするようにする。`URL.EscapedPath` の末尾が `/` の場合だけ末尾の `/` を 1 つ取り除き、`%2F` (または `%2f`) で終わるパスは取り除かない。あわせて、`RawPath` を持つパスの扱いと、`http.ServeMux` でのルート一致をテストで固定する。

## 現状

- `middleware.go` の `removeTrailingSlash` は、デコード済みの `URL.Path` の末尾が `/` であれば、`Path` と `RawPath` の両方から末尾の `/` を 1 つ取り除く。
- `/speech%2F` は、パースすると `Path` が `/speech/`、`RawPath` が `/speech%2F`、`RequestURI` が `/speech%2F` になる。
  - 現在の `removeTrailingSlash` は `Path` を `/speech` にするが、`RawPath` の末尾は `/` ではないため `/speech%2F` のまま変わらない。
  - `RawPath` が `Path` のエンコードとして正しくなくなるため、`URL.EscapedPath` は `Path` から組み立て直して `/speech` を返す。`http.ServeMux` は `EscapedPath` を `cleanPath` したうえでセグメント単位で照合するため、`/speech%2F` が `/speech` のルートに一致する。
  - echo の `RemoveTrailingSlash` を使っていた頃は、echo のルーターが `RawPath` で照合していたため 404 だった。`disable_trailing_slash_removal = true` の場合も 404 になる。
- `/speech/%2F` は、パースすると `Path` が `/speech//`、`RawPath` が `/speech/%2F`、`RequestURI` が `/speech/%2F` になる。
  - 現在の `removeTrailingSlash` は `Path` を `/speech/`、`RequestURI` を `/speech/` にし、エンコードされたスラッシュをリテラルのスラッシュに変える。
  - `RawPath` は `/speech/%2F` のままで、`Path` のエンコードとして正しくなくなる。
  - `/speech/%2F` は現在も `/speech` のルートに一致しない (404)。
- `middleware_test.go` の `TestRemoveTrailingSlash` は、エンコードを含まないパスだけを確認しており、`RawPath` が空でないパスを通らない。`http.ServeMux` を通したルート一致も確認していない。

### 再現手順

1. `disable_trailing_slash_removal` を既定 (false) にして `-service test` で起動する。
2. HTTP/2 で `POST /speech%2F` に `sora-audio-streaming-language-code: ja-JP` を付けて送る。
3. `/speech` のルートに一致し、404 ではなく 200 になる。本来は `/speech%2F` は `/speech` のルートに一致しないため、404 になるべき。

## 設計方針

- `removeTrailingSlash` は、変更前の `r.URL.EscapedPath()` の末尾が `/` の場合だけ、末尾の `/` を 1 つ取り除く。`EscapedPath` の末尾が `%2F` または `%2f` の場合は取り除かない。
  - `EscapedPath` の末尾が `/` なら `Path` も末尾が `/` である。`RawPath` が空でなく、`URL.EscapedPath()` が `RawPath` を返す場合、`RawPath` も末尾が `/` である。
  - `Path` が `/` の場合は取り除かない (現在の `len(path) <= 1` の判定を維持する)。
- `RawPath` が空でない場合は、`Path` と `RawPath` の両方から末尾の `/` を 1 つ取り除く。
  - `RawPath` が空でなく、`URL.EscapedPath()` が `RawPath` を返す場合、変更後も `URL.EscapedPath()` が `RawPath` を返す。
  - `RawPath` が正しくない場合 (生の `{` や空白などを含むリクエスト) は、変更後の正しさは保証しない。`URL.EscapedPath` は `Path` から組み立てるため、ルーティングには影響しない。
  - `RawPath` が空の場合は `Path` だけを取り除き、`RawPath` は空のままにする。
- 受け取ったリクエストは書き換えず、`r.WithContext` で浅いコピーを作り、コピーの `URL` と `RequestURI` を書き換える。`RawQuery` と `ForceQuery` は維持する。
- `TestRemoveTrailingSlash` に次のケースを追加し、`Path`、`RawPath`、`RequestURI` の期待値を確認する。元のリクエストの `Path`、`RawPath`、`RequestURI` が変わらないことも確認する。

| リクエスト | `Path` | `RawPath` | `RequestURI` | 確認内容 |
| --- | --- | --- | --- | --- |
| `/speech%2F` | `/speech/` | `/speech%2F` | `/speech%2F` | 末尾が `%2F` なので取り除かない |
| `/speech%2F/` | `/speech/` | `/speech%2F` | `/speech%2F` | 末尾のリテラルの `/` だけを取り除く |
| `/sp%65ech/` | `/speech` | `/sp%65ech` | `/sp%65ech` | エンコードを含む末尾スラッシュを取り除く |
| `/speech/%2F` | `/speech//` | `/speech/%2F` | `/speech/%2F` | 末尾が `%2F` なので取り除かない |
| `/speech/?` | `/speech` | (空) | `/speech?` | `ForceQuery` を維持する |

- `middleware_test.go` の `TestMainHandler` に `POST /speech` のルートを追加し、`newMainHandler(mux, true)` を通して次のルート一致を確認する。
  - `POST /speech` と `POST /speech/` は 200。
  - `POST /speech%2F` は 404。
  - `POST /speech%2F/` は 404。
  - `POST /sp%65ech/` は 200。
  - `POST /speech/%2F` は 404。
  - `POST /speech/?` は 200。
- `middleware_test.go` に `FuzzRemoveTrailingSlash` を追加する。入力は `/` で始まるエスケープ済みのパス (クエリを含んでよい) とし、`url.ParseRequestURI` で解析する。解析に失敗した入力はスキップする。次の性質を確認する。
  - 除去後の `EscapedPath` を `url.PathUnescape` すると `Path` に一致する。
  - 除去後の `RequestURI` を `url.ParseRequestURI` で解析すると、`Path` が一致する。
  - 元の `RawPath` が空でなく、`URL.EscapedPath()` が `RawPath` を返す場合、除去後の `RawPath` も `URL.EscapedPath()` が `RawPath` を返す。
  - 元のリクエストの `Path`、`RawPath`、`RequestURI` が変わらない。
- シードには、`/`、`/speech/`、`/speech//`、`/speech%2F`、`/speech%2F/`、`/sp%65ech/`、`/speech/%2F`、`/speech/?` を含める。
- `make test` で単体テストと fuzz テストのシードが通り、`go test -fuzz=FuzzRemoveTrailingSlash` で fuzz を実行できることを確認する。
- `CHANGES.md` の `## develop` に、`[FIX]` として「`/speech%2F` のようなパーセントエンコードされたスラッシュで終わるパスを、末尾スラッシュの除去の対象外にする」ことを記載する。

## 完了条件

- `/speech%2F` が `/speech` のルートに一致しない。
- `/speech%2F/` が `/speech` のルートに一致しない。
- `/sp%65ech/` が `/speech` のルートに一致する (これまでどおり)。
- `/speech/%2F` が `/speech` のルートに一致しない (現状と同じ)。
- `/speech/?` が `/speech` のルートに一致する (クエリを維持する)。
- `TestRemoveTrailingSlash` が、上記の `Path`、`RawPath`、`RequestURI` の期待値と、元のリクエストが書き換わらないことを確認する。
- `TestMainHandler` のルート一致テストが、上記のステータスコードを確認する。
- `FuzzRemoveTrailingSlash` があり、上記の性質を確認する。シードが `go test` で通り、`go test -fuzz=FuzzRemoveTrailingSlash` で fuzz を実行できる。
- `make test` が通る。
- `CHANGES.md` の `## develop` に `[FIX]` が追加されている。
