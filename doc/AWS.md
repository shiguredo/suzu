# Amazon Transcribe

## 音声の解析結果が 2 つ返ってくる

`config.toml` で `aws_enable_channel_identification` を `false` にし、
`audio_channel_count` を 1 に設定してください。

