# ヘッダー付きのパケットを組み立てる処理の重複を整理する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/refactor-options-packet-reader
- Polished: 2026-11-25

## 目的

`options.go` の `optionReadPacketWithHeader` のパースの処理を 1 つにまとめ、読みやすく変更しやすい形にする。挙動は変えない。

## 現状

- `optionReadPacketWithHeader` は、受信したデータに 1 つ目のパケットが揃ったときの処理と、残りのデータに次のパケットが含まれる場合の内側のループで、同じパースの処理 (ヘッダーの長さの確認、length の取り出し、`MaxPayloadLength` の確認、ペイロードの送信) を 2 回書いている。
- `ErrPayloadTooLarge` は `"PAYLOAD-TOO-LARGE: %d"` という書式の文字列だが、エラーの値のような `Err` の接頭辞が付いている。`errors.Is` で判定できない。

## 設計方針

- 「バッファに揃っているパケットをすべて送り出す」処理を 1 つのループにまとめる。
- `PAYLOAD-TOO-LARGE` はエラーの値として定義し、`fmt.Errorf("%w: %d", ...)` で長さを付ける。
- `handler_test.go` の `TestReadPacketWithHeader` と `options_test.go` のテストが、検証内容を変えずに通ることを確認する。

## 完了条件

- パースの処理が 1 箇所になり、既存のテストが通る。
- `PAYLOAD-TOO-LARGE` のエラーを `errors.Is` で判定できる。
