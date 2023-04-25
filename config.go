package suzu

import (
	"gopkg.in/ini.v1"
)

const (
	// 100ms
	DefaultTimeToWaitForOpusPacketMs = 100
)

type Config struct {
	Revision string

	Debug bool `ini:"debug"`

	ListenAddr string `ini:"listen_addr"`
	ListenPort int    `ini:"listen_port"`

	HTTP2FullchainFile    string `ini:"http2_fullchain_file"`
	HTTP2PrivkeyFile      string `ini:"http2_privkey_file"`
	HTTP2VerifyCacertPath string `ini:"http2_verify_cacert_path"` // クライアント認証用

	HTTP2MaxConcurrentStreams uint32 `ini:"http2_max_concurrent_streams"`
	HTTP2MaxReadFrameSize     uint32 `ini:"http2_max_read_frame_size"`
	HTTP2IdleTimeout          uint32 `ini:"http2_idle_timeout"`

	Retry *bool `ini:"retry"`

	ExporterIPAddress string `ini:"exporter_ip_address"`
	ExporterPort      int    `ini:"exporter_port"`

	SkipBasicAuth     bool   `ini:"skip_basic_auth"`
	BasicAuthUsername string `ini:"basic_auth_username"`
	BasicAuthPassword string `ini:"basic_auth_password"`

	SampleRate   int `ini:"audio_sample_rate"`
	ChannelCount int `ini:"audio_channel_count"`

	DumpFile string `ini:"dump_file"`

	LogDir    string `ini:"log_dir"`
	LogName   string `ini:"log_name"`
	LogDebug  bool   `ini:"log_debug"`
	LogStdout bool   `ini:"log_stdout"`

	TimeToWaitForOpusPacketMs int `ini:"time_to_wait_for_opus_packet_ms"`

	// aws の場合は IsPartial が false, gcp の場合は IsFinal が true の場合にのみ結果を返す指定
	FinalResultOnly bool `ini:"final_result_only"`

	// Amazon Web Services
	AwsCredentialFile                    string `ini:"aws_credential_file"`
	AwsProfile                           string `ini:"aws_profile"`
	AwsRegion                            string `ini:"aws_region"`
	AwsEnablePartialResultsStabilization bool   `ini:"aws_enable_partial_results_stabilization"`
	AwsPartialResultsStability           string `ini:"aws_partial_results_stability"`
	AwsEnableChannelIdentification       bool   `ini:"aws_enable_channel_identification"`
	// 変換結果に含める項目の有無の指定
	AwsResultChannelID bool `ini:"aws_result_channel_id"`
	AwsResultIsPartial bool `ini:"aws_result_is_partial"`

	// Google Cloud Platform
	GcpCredentialFile                      string   `ini:"gcp_credential_file"`
	GcpEnableSeparateRecognitionPerChannel bool     `ini:"gcp_enable_separate_recognition_per_channel"`
	GcpAlternativeLanguageCodes            []string `ini:"gcp_alternative_language_codes"`
	GcpMaxAlternatives                     int32    `ini:"gcp_max_alternatives"`
	GcpProfanityFilter                     bool     `ini:"gcp_profanity_filter"`
	GcpEnableWordTimeOffsets               bool     `ini:"gcp_enable_word_time_offsets"`
	GcpEnableWordConfidence                bool     `ini:"gcp_enable_word_confidence"`
	GcpEnableAutomaticPunctuation          bool     `ini:"gcp_enable_automatic_punctuation"`
	GcpEnableSpokenPunctuation             bool     `ini:"gcp_enable_spoken_punctuation"`
	GcpEnableSpokenEmojis                  bool     `ini:"gcp_enable_spoken_emojis"`
	GcpModel                               string   `ini:"gcp_model"`
	GcpUseEnhanced                         bool     `ini:"gcp_use_enhanced"`
	GcpSingleUtterance                     bool     `ini:"gcp_single_utterance"`
	GcpInterimResults                      bool     `ini:"gcp_interim_results"`
	// 変換結果に含める項目の有無の指定
	GcpResultIsFinal   bool `ini:"gcp_result_is_final"`
	GcpResultStability bool `ini:"gcp_result_stability"`
}

func InitConfig(data []byte, config *Config) error {
	f, err := ini.InsensitiveLoad(data)
	if err != nil {
		// パースに失敗した場合 Fatal で終了
		return err
	}

	if err := f.MapTo(config); err != nil {
		// マッピングに失敗した場合 Fatal で終了
		return err
	}

	if config.TimeToWaitForOpusPacketMs == 0 {
		config.TimeToWaitForOpusPacketMs = DefaultTimeToWaitForOpusPacketMs
	}

	// 未指定の場合は true
	if config.Retry == nil {
		defaultRetry := true
		config.Retry = &defaultRetry
	}

	// TODO(v): 初期値
	return nil
}
