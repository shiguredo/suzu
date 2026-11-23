# 末尾スラッシュの除去がパーセントエンコードされたスラッシュ (%2F) も取り除く

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-trailing-slash-removal-encoded-slash
- Polished: {YYYY-MM-DD}

## 目的

末尾スラッシュの除去 (`disable_trailing_slash_removal` が無効な場合の既定の挙動) が、リテラルのスラッシュだけを対象にするようにする。あわせて、RawPath を持つパスの扱いをテストで固定する。

## 現状

- `middleware.go` の `removeTrailingSlash` は、デコード済みの `URL.Path` の末尾が `/` であれば、`Path` と `RawPath` の両方から末尾の `/` を 1 つ取り除く。
- `/speech%2F` は、`Path` が `/speech/`、`RawPath` が `/speech%2F` になる。
  - `Path` は `/speech` になるが、`RawPath` の末尾は `/` ではないため変わらない。
  - `RawPath` が `Path` のエンコードとして正しくなくなるため、`URL.EscapedPath` は `Path` から組み立て直して `/speech` を返す。`http.ServeMux` は `EscapedPath` でルートと照合するため、`/speech%2F` が `/speech` のルートに一致する。
  - echo の `RemoveTrailingSlash` を使っていた頃は、echo のルーターが `RawPath` で照合していたため 404 だった。`disable_trailing_slash_removal = true` の場合も 404 になる。
- `/speech/%2F` は、`Path` が `/speech/` に、`RequestURI` が `/speech/` になり、エンコードされたスラッシュがリテラルのスラッシュに変わる。
- `middleware_test.go` の `TestRemoveTrailingSlash` は、エンコードを含まないパスだけを確認しており、`RawPath` を扱う処理を通らない。

## 設計方針

- 末尾のスラッシュは、エンコードされたパス (`URL.EscapedPath`) の末尾が `/` の場合だけ取り除く。`%2F` で終わるパスは取り除かない。
- `RawPath` がある場合は、`RawPath` と `Path` の両方から末尾の `/` を取り除き、`RawPath` が `Path` のエンコードとして正しいままになるようにする。
- `TestRemoveTrailingSlash` に、`/speech%2F`、`/speech%2F/`、`/sp%65ech/`、`/speech/%2F`、`/speech/?` を追加する。
- Go 標準の fuzzing で、任意のパスに対して次が成り立つことを確認する。
  - 除去した後の `EscapedPath` をデコードすると `Path` に一致する。
  - 除去した後の `RequestURI` を解析し直すと、同じ `Path` になる。
  - 受け取ったリクエストを書き換えない。

## 完了条件

- `/speech%2F` が `/speech` のルートに一致しない。
- `/sp%65ech/` のようにエンコードを含む末尾スラッシュ付きのパスは、これまでどおり末尾スラッシュなしのルートとして受け付ける。
- 上記を確認する単体テストと fuzz テストがある。
