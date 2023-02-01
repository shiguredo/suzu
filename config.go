package suzu

import (
	"github.com/BurntSushi/toml"
)

const (
	// 100ms
	DefaultTimeToWaitForOpusPacketMs = 100
)

type Config struct {
	Revision string

	Debug bool `toml:"debug"`

	ListenAddr string `toml:"listen_addr"`
	ListenPort int    `toml:"listen_port"`

	HTTP2FullchainFile    string `toml:"http2_fullchain_file"`
	HTTP2PrivkeyFile      string `toml:"http2_privkey_file"`
	HTTP2VerifyCacertPath string `toml:"http2_verify_cacert_path"` // クライアント認証用

	HTTP2MaxConcurrentStreams uint32 `toml:"http2_max_concurrent_streams"`
	HTTP2MaxReadFrameSize     uint32 `toml:"http2_max_read_frame_size"`
	HTTP2IdleTimeout          uint32 `toml:"http2_idle_timeout"`

	ExporterIPAddress string `toml:"exporter_ip_address"`
	ExporterPort      int    `toml:"exporter_port"`

	SkipBasicAuth     bool   `toml:"skip_basic_auth"`
	BasicAuthUsername string `toml:"basic_auth_username"`
	BasicAuthPassword string `toml:"basic_auth_password"`

	SampleRate   int `toml:"audio_sample_rate"`
	ChannelCount int `toml:"audio_channel_count"`

	DumpFile string `toml:"dump_file"`

	LogDir    string `toml:"log_dir"`
	LogName   string `toml:"log_name"`
	LogDebug  bool   `toml:"log_debug"`
	LogStdout bool   `toml:"log_stdout"`

	TimeToWaitForOpusPacketMs int `toml:"time_to_wait_for_opus_packet_ms"`

	// Amazon Web Services
	AwsCredentialFile                    string `toml:"aws_credential_file"`
	AwsProfile                           string `toml:"aws_profile"`
	AwsRegion                            string `toml:"aws_region"`
	AwsEnablePartialResultsStabilization bool   `toml:"aws_enable_partial_results_stabilization"`
	AwsPartialResultsStability           string `toml:"aws_partial_results_stability"`
	AwsEnableChannelIdentification       bool   `toml:"aws_enable_channel_identification"`
	// 変換結果に含める項目の有無の指定
	AwsResultChannelID bool `toml:"aws_result_channel_id"`
	AwsResultIsPartial bool `toml:"aws_result_is_partial"`

	// Google Cloud Platform
	GcpCredentialFile                      string   `toml:"gcp_credential_file"`
	GcpEnableSeparateRecognitionPerChannel bool     `toml:"gcp_enable_separate_recognition_per_channel"`
	GcpAlternativeLanguageCodes            []string `toml:"gcp_alternative_language_codes"`
	GcpMaxAlternatives                     int32    `toml:"gcp_max_alternatives"`
	GcpProfanityFilter                     bool     `toml:"gcp_profanity_filter"`
	GcpEnableWordTimeOffsets               bool     `toml:"gcp_enable_word_time_offsets"`
	GcpEnableWordConfidence                bool     `toml:"gcp_enable_word_confidence"`
	GcpEnableAutomaticPunctuation          bool     `toml:"gcp_enable_automatic_punctuation"`
	GcpEnableSpokenPunctuation             bool     `toml:"gcp_enable_spoken_punctuation"`
	GcpEnableSpokenEmojis                  bool     `toml:"gcp_enable_spoken_emojis"`
	GcpModel                               string   `toml:"gcp_model"`
	GcpUseEnhanced                         bool     `toml:"gcp_use_enhanced"`
	GcpSingleUtterance                     bool     `toml:"gcp_single_utterance"`
	GcpInterimResults                      bool     `toml:"gcp_interim_results"`
	// 変換結果に含める項目の有無の指定
	GcpResultIsFinal   bool `toml:"gcp_result_is_final"`
	GcpResultStability bool `toml:"gcp_result_stability"`
}

func InitConfig(data []byte, config *Config) error {
	if err := toml.Unmarshal(data, config); err != nil {
		// パースに失敗した場合 Fatal で終了
		return err
	}

	if config.TimeToWaitForOpusPacketMs == 0 {
		config.TimeToWaitForOpusPacketMs = DefaultTimeToWaitForOpusPacketMs
	}

	// TODO(v): 初期値
	return nil
}
