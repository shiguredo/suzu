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

### misc

## 2024.5.1

- [FIX] リリース時の build 前に patch をあてるように修正する
  - @Hexa
- [ADD] リリース時の build を Makefile にまとめる
  - @Hexa

## 2024.5.0

- [FIX] 高ビットレートの音声データの場合に、解析結果が送られてこない不具合を修正する
  - @Hexa

## 2024.4.0

- [ADD] audio streaming header に対応する
  - @Hexa
- [ADD] クライアントから送られてくるデータにヘッダーが付与されている場合に対応する audio_streaming_header 設定を追加する
  - デフォルト値: false
  - @Hexa
- [CHANGE] silent packet の送信までのデフォルトの時間を 10 秒に変更する
  - @Hexa

## 2024.3.0

- [ADD] Amazon Transcribe からの結果の Results[].ResultId をクライアントに返す aws_result_id 設定を追加する
  - デフォルト値: false
  - @Hexa

## 2024.2.0

- [CHANGE] retry 設定を削除し、リトライ回数を指定する max_retry 設定を追加する
  - リトライしない場合は、max_retry を設定ファイルから削除するか、または、max_retry = 0 を設定する
  - デフォルト値: 0 （リトライ無し）
  - @Hexa
- [ADD] サービス接続時のエラーによるリトライまでの時間間隔を指定する retry_interval_ms 設定（ミリ秒間隔）を追加する
  - デフォルト値: 100 （100 ms）
  - @Hexa
- [ADD] サービス接続時の特定のエラー発生時に、リトライする仕組みを追加する
  - @Hexa
- [ADD] ハンドラーにリトライ回数を管理するメソッドを追加する
  - @Hexa
- [CHANGE] aws への接続時に、時間をおいて再接続できる可能性がある HTTP ステータスコードが 429 の応答の場合は、指定されたリトライ設定に応じて、再接続を試みるように変更する
  - @Hexa
- [CHANGE] aws、または、gcp への接続後にリトライ回数が max_retry を超えた場合は、{"type": "error", "reason": string} をクライアントへ送信する
  - @Hexa

## 2024.1.0

- [UPDATE] go.mod の Go のバージョンを 1.22.0 にあげる
  - @voluntas
- [CHANGE] サービス接続時にエラーになった場合は、Body が空のレスポンスを返すように変更する
  - @Hexa
- [CHANGE] サービス接続後にエラーになった場合は、{"type": "error", "reason": string} をクライアントへ送信するように変更する
  - @Hexa
- [CHANGE] aws の再接続条件の exception から InternalFailureException を削除する
  - @Hexa

## 2023.5.3

- [FIX] VERSION ファイルを tag のバージョンに修正する
  - @Hexa

## 2023.5.2

- [UPDATE] go.mod の Go のバージョンを 1.21.5 にあげる
  - @voluntas
- [FIX] stream 処理開始後のエラー時に、ログに出力される status code が実際にクライアントに送信した値になるように修正する
  - @Hexa

## 2023.5.1

- [FIX] HTTP/2 Rapid Reset 対策として Go 1.21.3 以上でリリースバイナリを作成するよう修正する
  - <https://groups.google.com/g/golang-announce/c/iNNxDTCjZvo>
  - @voluntas

## 2023.5.0

- [ADD] -V で VERSION ファイルを表示する
  - @voluntas
- [ADD] VERSION ファイルを追加する
  - @voluntas
- [ADD] `h2c` を有効にするため `https` を追加する
  - @voluntas
- [ADD] exporter で HTTPS を有効にする `exporter_https` を追加する
  - @voluntas
- [CHANGE] コンソールログの日付フォーマットを修正する
  - @voluntas
- [CHANGE] lumberjack を公式に戻す
  - @voluntas
- [UPDATE] go.mod, Github Actions で使用する Go のバージョンを 1.21.0 にあげる
  - @Hexa

## 2023.4.0

- [CHANGE] サンプル設定ファイル名を変更する
  - @voluntas
- [CHANGE] 起動時のポート番号の表示を削除する
  - @Hexa
- [FIX] 標準出力へのログ出力の書式を修正する
  - @Hexa
- [FIX] サポート外の Language Code が送られてきた場合のエラーを修正する
  - @Hexa
- [FIX] HTTP リクエストの待ち受けアドレスの指定に、listen_addr の値を使用するように修正する
  - @Hexa

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
