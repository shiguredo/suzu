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

- [CHANGE] サンプル設定ファイル名を変更する
    - @voluntas

## 2023.3.0

- [ADD] 受信した解析結果を操作できる OnResultFunc を追加する
    - @Hexa
- [CHANGE] サービス毎に OnResultFunc 処理を指定できるようにするために Handler を struct に変更する
    - @Hexa
- [ADD] 受信した最終的な解析結果のみをクライアントに返す処理と、この処理を有効にする設定を追加する
    - final_result_only = true を設定した場合は、解析結果が下記の場合の結果を返さない
        - AWS の場合: is_partial = true の場合はクライアントに結果を返さない
        - GCP の場合: is_final = false の場合はクライアントに結果を返さない
    - @Hexa
- [FIX] パケット読み込みの停止時に goroutine が停止しない場合がある処理を修正する
    - @Hexa
- [CHANGE] 設定ファイルを toml から ini に変更する
    - @Hexa
- [CHANGE] ログの key を snake case で統一する
    - @Hexa
- [UPDATE] 使用していない log_debug を削除する
    - @Hexa
- [CHANGE] http2_fullchain_file, http2_privkey_file, http2_verify_cacert_path の設定を tls_fullchain_file, tls_privkey_file, tls_verify_cacert_path に変更する
    - @Hexa
- [ADD] log_rotate 関連の設定を追加する
    - @Hexa
- [ADD] Pion の oggwriter.go を追加する
    - @Hexa
- [CHANGE] HTTP リクエストのログを zerolog の書式で出力するように変更する
    - @Hexa
- [CHANGE] debug=false で log_stdout=true の場合は標準出力に JSON でログ出力させる
    - @Hexa
- [FIX] ログ出力時のタイムスタンプを UTC に修正する
    - @Hexa
- [CHANGE] リリースバイナリの作成時には CGO_ENABLED=0 を指定する
    - @Hexa

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

