# Suzu の使い方

## ビルド

```console
$ make
```

## 設定ファイル

config_example.ini をコピーして、AWS や GCP の設定をしてください

```console
$ cp config_example.ini config.ini
```

## 開発環境での利用

HTTPS 設定無効にすることで HTTP/2 over TCP (h2c) での通信が利用できます。
この場合は証明書の設定は不要です。

### Suzu 側の設定

```ini
https = false
```

### Sora 側の設定

HTTP にすることで HTTP/2 over TCP (h2c) を利用して接続しに行きます。

```ini
audio_streaming_url = http://192.0.2.10:5890/speech
```

## Amazon Transcribe を利用する

-service で `aws` を指定することで AWS Transcribe が利用されます。

```
$ ./bin/suzu -C config.ini -service aws
```

AWS Transcribe を利用するに当たっての注意事項は [AWS.md](AWS.md) をご確認ください。

## Google Speech To Text を利用する

-service で `gcp` を指定することで GCP Speech-to-Text が利用されます。

```
$ ./suzu -C config.ini -service gcp
```

GCP Speech-to-Text を利用するに当たっての注意事項は [GCP.md](GCP.md) をご確認ください。

## デバッグ機能

### /test

実際に音声解析サーバーにパケットを流しません。送られてきた音声のバイナリサイズをプッシュ通知で送り返します。

### /dump

この URL を Sora の audio_streaming_url を指定すると、
音声ストリミーングに流れてくる音声データを JSON 形式でダンプします。
