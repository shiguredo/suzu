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

## 2026.1.0

**リリース日: 2026-01-15

- [CHANGE] AWS SDK for Go v1 がサポート終了したため、このバージョンの SDK を使用していた awsv1 を削除する
  - @Hexa
- [CHANGE] 意図的な停止のため、context canceled による停止時は、ログを出さないように変更する
  - @Hexa
- [FIX] GOAWAY なしでクライアントとの接続が切断された場合に、goroutine が停止しない場合がある処理を修正する
  - @Hexa
- [FIX] gcp 指定時の切断時の client.Close 抜けを修正する
  - @Hexa

### misc

- [UPDATE] actions/checkout を v5 に上げる
  - @miosakuma

## 2025.5.0

**リリース日: 2025-07-09

- [UPDATE] サービスへの接続処理の前後で、クライアントの接続を識別可能な情報を付加したログを出力する
  - 識別情報として、クライアントから受信した sora-channel-id ヘッダーおよび sora-connection_id ヘッダー、また、AWS から受信した x-amzn-transcribe-session-id ヘッダーを出力
  - @Hexa

## 2025.4.1

**リリース日: 2025-06-23

- [FIX] 音声データ送信時に毎回ヘッダを送信していた不具合を修正する
  - @Hexa


## 2025.4.0

**リリース日: 2025-05-22

- [ADD] silent packet を無効にする `disable_silent_packet` の設定を追加する
  - デフォルトは `false`
  - true にした場合は、クライアントから音声データが送られてこない場合に、suzu から silent packet をサーバに送らない
    - そのため、クライアントから音声データが送られてこない場合には、一定時間でサーバから切断されることを想定している
  - @Hexa
- [ADD] クライアントから音声データが送られてこない場合に、suzu からサーバに silent packet を送信する際の送信間隔を指定する `time_to_wait_for_opus_packet_ms` の設定を追加する
  - デフォルトは `10000` （10 秒）
  - この設定値は `disable_silent_packet` が `false` に設定されている場合にのみ有効
  - @Hexa
- [CHANGE] リトライ時にクライアントから音声データが送られてきてからサーバに接続するように変更する
  - これまでは、suzu でのリトライ時にクライアントから音声データが送られてくる前にサーバに接続していたが、クライアントから音声データを受信してからサーバにリクエストを送信するように変更する
  - @Hexa

## 2025.3.0

**リリース日: 2025-05-09

- [UPDATE] aws, awsv1, awsv2 指定時に、接続時にサーバから送られてきた Session-Id をログ出力に追加する
  - @Hexa
- [ADD] リリースに linux arm64 を追加する
  - @voluntas

## 2025.2.0

**リリース日: 2025-05-08

- [ADD] リトライ対象の固定のエラー以外に、config.ini に設定したメッセージに該当するエラーメッセージ受信時にもリトライ対象とする `retry_targets` の設定を追加する
  - デフォルトは "" （未指定）
  - 対象のエラーメッセージを複数指定する場合はカンマ（,）区切りで指定する
    - 例: retry_targets = ERROR1,ERROR2,ERROR3
  - @Hexa

## 2025.1.0

**リリース日: 2025-05-07

- [ADD] ログのメッセージキー名を指定する `log_message_key_name` を追加する
  - デフォルトは `message`
  - @voluntas
- [ADD] ログのタイムスタンプキー名を指定する `log_timestamp_key_name` を追加する
  - デフォルトは `time`
  - @voluntas

### misc

- [CHANGE] GitHub Actions の ubuntu-latest を ubuntu-24.04 に変更する
  - @voluntas
- [UPDATE] go.mod の Go のバージョンを 1.24.2 にあげる
  - @Hexa @voluntas
- [UPDATE] GitHub Actions の staticcheck のバージョンを 2025.1.1 に上げる
  - @Hexa @voluntas

## 2024.11.0

- [CHANGE] suzu 実行時に指定する -service オプションのデフォルト値の **aws** で使用する AWS SDK for Go を、AWS SDK for Go v1 から AWS SDK for Go v2 に変更する
  - 以前のバージョンと同様に AWS SDK for Go v1 を使用する場合は、-service オプションで **awsv1** を指定する
  - awsv1 は、AWS SDK for Go v1 のサポート終了の 2025-07-31 を目処に廃止します
  - @Hexa
- [CHANGE] awsv1 指定時に、Region または Endpoint が見つからかなった場合は再接続は困難とみなし、クライアントへ {"type": "error", "reason": string} を送信して処理を終了するように変更する
  - @Hexa
- [CHANGE] aws, awsv2 指定時に、Region または Endpoint が見つからない等の OperationError が発生した場合は再接続は困難とみなし、クライアントへ {"type": "error", "reason": string} を送信して処理を終了するように変更する
  - @Hexa
- [CHANGE] gcp 指定時に、speech client 作成時にエラーが発生した場合は再接続は困難とみなし、クライアントへ {"type": "error", "reason": string} を送信して処理を終了するように変更する
  - @Hexa
- [ADD] `"domain": "suzu"` をログに含めるようにする
  - 複数のログを標準出力する際に判別できるようにする
  - @voluntas
- [ADD] デバッグコンソールログを出力する `debug_console_log` を追加する
  - デフォルト false
  - `debug` が `true` かつ `debug_console_log` が `true` の場合は、コンソールログにデバッグログを出力する
  - @voluntas
- [ADD] デバッグコンソールログを JSON 形式で出力する `debug_console_log_json` を追加する
  - デフォルトは false
  - @voluntas
- [ADD] ログローテーション時に圧縮するかをどうかを指定する `log_rotate_compress` を追加する
  - デフォルトは false
  - @voluntas
- [ADD] 受信した音声データを Ogg ファイルで保存するかを指定する `enable_ogg_file_output` を追加する
  - 保存するファイル名は、sora-session-id ヘッダーと sora-connection-id ヘッダーの値を使用して作成する
    - ${sora-session-id}-${sora-connection-id}.ogg
  - デフォルト値: false
  - @Hexa
- [ADD] 受信した音声データを Ogg ファイルで保存する場合の保存先ディレクトリを指定する ogg_dir を追加する
  - デフォルト値: .
  - @Hexa
- [ADD] AWS SDK for Go v2 対応を追加する
  - suzu 実行時に -service オプションを未指定にするか、-service オプションで awsv2、または、aws を指定すると AWS SDK for Go v2 を使用する
    - 実行例: ./bin/suzu -service awsv2
    - 実行例: ./bin/suzu -service aws
  - @Hexa
- [FIX] aws, awsv2 指定時に、config.ini に aws_profile が指定されていない場合でも、config.ini に指定された aws_region を使用するように修正する
  - @Hexa

## 2024.10.0

- [CHANGE] Amazon Transcribe 向けの minimum_confidence_score と minimum_transcribed_time が両方ともに無効（0）に設定されていた場合は、フィルタリングしない結果を返すように変更する
  - @Hexa

## 2024.9.0

- [CHANGE] Amazon Transcribe 向けの minimum_confidence_score と minimum_transcribed_time を独立させて、minimum_confidence_score が無効でも minimum_transcribed_time が有効な場合は minimum_transcribed_time でのフィルタリングが有効になるように変更する
  - @Hexa
- [CHANGE] フィルタリングの結果が句読点のみになった場合はクライアントに結果を返さないように変更する
  - @Hexa
- [CHANGE] サーバから切断された場合はリトライするように変更する
  - Amazon Transcribe のみ対象
  - @Hexa

## 2024.8.0

- [ADD] 採用する結果の信頼スコアの最小値を指定する minimum_confidence_score を追加する
  - Amazon Transcribe のみ有効
  - デフォルト値: 0（信頼スコアを無視する）
  - @Hexa
- [ADD] 採用する結果の最小発話期間（秒）を指定する minimum_transcribed_time を追加する
  - Amazon Transcribe のみ有効
  - デフォルト値: 0（最小発話期間を無視する）
  - @Hexa

## 2024.7.0

- [FIX] サービスへの接続が成功してもリトライカウントがリセットされない不具合を修正する
  - @Hexa
- [FIX] 解析結果だけでなくエラーメッセージの送信時にもリトライカウントをリセットしていたため、リトライ処理によってカウントがリセットされていた不具合を修正する
  - @Hexa
- [FIX] リトライ待ち時にクライアントから切断しようとすると、リトライ待ちで処理がブロックされているため切断までに時間がかかる不具合を修正する
  - @Hexa

## 2024.6.0

- [CHANGE] aws の再接続条件の exception に InternalFailureException を追加する
  - @Hexa

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
