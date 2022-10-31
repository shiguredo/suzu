# Audio Streaming Gateway Suzu


**現在リリースに向けて開発中です**

[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/shiguredo/suzu.svg)](https://github.com/shiguredo/suzu)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## About Shiguredo's open source software

We will not respond to PRs or issues that have not been discussed on Discord. Also, Discord is only available in Japanese.

Please read https://github.com/shiguredo/oss/blob/master/README.en.md before use.

## 時雨堂のオープンソースソフトウェアについて

利用前に https://github.com/shiguredo/oss をお読みください。

## Audio Streaming Gateway Suzu について

Suzu は [WebRTC SFU Sora](https://sora.shiguredo.jp) から音声データを HTTP/2 経由で受け取り、
音声解析サービスへ送信し解析結果を Sora へ戻すゲートウェイです。

## 目的

リアルタイム通話で気軽に音声解析サービスを利用できる仕組みを提供する事です。

## 特徴

- Sora から音声データを HTTP/2 経由で受け取り、音声解析サービスへ送信します
- 音声解析サービスの解析結果を HTTP/2 レスポンスで Sora に戻します
- Sora は受け取った解析結果を DataChannel 経由でクライアントへプッシュで送信します
- 音声解析に必要とされる言語コードをクライアント事に指定可能です
- mTLS 対応

## 使ってみる

TBD

<!---
Suzu を使ってみたい人は [USE.md](doc/USE.md) をお読みください。
-->

## 対応サービス

- [x] [Amazon Transcribe](https://aws.amazon.com/jp/transcribe/)
- [ ] [Google Cloud Speech-to-Text](https://cloud.google.com/speech-to-text)
- [ ] [Microsoft Azure Speech to Text](https://azure.microsoft.com/ja-jp/products/cognitive-services/speech-to-text/)
- [ ] [Microsoft Azure Speech Translation](https://azure.microsoft.com/ja-jp/products/cognitive-services/speech-translation/)
- [ ] [Deepgram](https://deepgram.com/)
- [ ] [AmiVoice Cloud Platform](https://acp.amivoice.com/amivoice/)

## ライセンス

```
Copyright 2022-2022, Hiroshi Yoshida (Original Author)
Copyright 2022-2022, Shiguredo Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```

## 優先実装

優先実装とは Sora のライセンスを契約頂いているお客様限定で Suzu の実装予定機能を有償にて前倒しで実装することです。

### 優先実装が可能な機能一覧

詳細は Discord やメールなどでお気軽にお問い合わせください。

- [Google Cloud Speech-to-Text](https://cloud.google.com/speech-to-text) 対応
- [Microsoft Azure Speech to Text](https://azure.microsoft.com/ja-jp/products/cognitive-services/speech-to-text/) 対応
- [Microsoft Azure Speech Translation](https://azure.microsoft.com/ja-jp/products/cognitive-services/speech-translation/) 対応
- [Deepgram](https://deepgram.com/) 対応
- [AmiVoice Cloud Platform](https://acp.amivoice.com/amivoice/) 対応
- ウェブフック機能対応
    - クライアント事に接続先サービスを変更できるようになる