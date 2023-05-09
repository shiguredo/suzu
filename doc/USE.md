# 使ってみる

## パッケージダウンロード



## 設定ファイル

サンプルをコピーしてください。

```
$ cp config_example.ini config.ini
```


## Amazon Transcribe を利用する

-service で `aws` を指定することで AWS Transcribe が利用されます。

```
$ ./suzu -C config.ini -service aws
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
