## 開発ビルド

初回だけ make init と設定を cp する必要があります。

```console
$ make init
$ cp config.example.yaml config.yaml
```

air をインストールして使ってください。

[cosmtrek/air: ☁️ Live reload for Go apps](https://github.com/cosmtrek/air)

```console
$ air
```

## ビルド

初回だけ make init と設定を cp する必要があります。

```console
$ make init
$ cp config.example.yaml config.yaml
```

ビルドすれば bin 以下にバイナリが生成されます。

```console
$ make
```

