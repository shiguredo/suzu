# 変更履歴

- CHANGE
    - 下位互換のない変更
- UPDATE
    - 下位互換がある変更
- ADD
    - 下位互換がある追加
- FIX
    - バグ修正

## develop

## 2023.2.0

- [CHANGE] log_stdout = true の場合はログをファイルに出力しないように変更する
    - @Hexa
- [UPDATE] パッチで変更していた使用していないメソッド内の変更を削除する
    - @Hexa
- [CHANGE] GCP を使用する場合のクライアントに送信される結果から channel_id の項目を削除する
    - @Hexa
- [CHANGE] 設定ファイルで指定されている音声解析サービスからの結果項目をクライアントへ送信する結果に含める
    - 現在設定ファイルで指定可能な項目は下記の通り
        - AWS: channel_id, is_partial
        - GCP: is_final, stability
    - @Hexa
- [UPDATE] go.mod, Github Actions で使用する Go のバージョンを 1.20 にあげる
    - @Hexa
- [UPDATE] Github Actions で使用する staticcheck のバージョンを 2023.1.2 にあげる
    - @Hexa
- [CHANGE] 特定のエラーでサーバから切断された際に再度接続する処理を追加する
    - 再接続対象のエラーは下記の通り
        - AWS: LimitExceededException, InternalFailureException
        - GCP: OutOfRange, InvalidArgument, ResourceExhausted
    - @Hexa
- [ADD] サーバから切断された際に再度接続する処理の有無を指定する設定を追加する
    - @Hexa

## 2023.1.0

**祝リリース**

