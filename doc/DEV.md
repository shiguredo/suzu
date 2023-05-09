# 開発者向け

## セットアップ

初回だけ make init と設定を cp する必要があります。

```console
$ make init
$ cp config_example.ini config.ini
```

## リリースビルド

```console
$ make
```

bin 以下にバイナリが生成されます。

## ライブラリロード

air をインストールして使ってください。

[cosmtrek/air: ☁️ Live reload for Go apps](https://github.com/cosmtrek/air)

```console
$ air
```

