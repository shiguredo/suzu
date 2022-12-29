# 使ってみる

## パッケージダウンロード



## 設定ファイル

サンプルをコピーしてください。

```
$ cp config.example.toml config.toml
```


## Amazon Transcribe を利用する

-service で `aws` を指定することで AWS Transcribe が利用されます。

```
$ ./suzu -C config.toml -service aws
```

## Google Speech To Text を利用する

-service で `gcp` を指定することで GCP Speech-to-Text が利用されます。

```
$ ./suzu -C config.toml -service gcp
```

## デバッグ機能

### /test

実際に音声解析サーバーにパケットを流しません。送られてきた音声のバイナリサイズをプッシュ通知で送り返します。

### /dump

この URL を Sora の audio_streaming_url を指定すると、
音声ストリミーングに流れてくる音声データを JSON 形式でダンプします。
