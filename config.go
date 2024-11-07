package suzu

import (
	_ "embed"
	"fmt"
	"net/netip"

	zlog "github.com/rs/zerolog/log"
	"gopkg.in/ini.v1"
)

//go:embed VERSION
var Version string

const (
	DefaultLogDir  = "."
	DefaultLogName = "suzu.jsonl"

	// megabytes
	DefaultLogRotateMaxSize    = 200
	DefaultLogRotateMaxBackups = 7
	// days
	DefaultLogRotateMaxAge = 30

	DefaultExporterListenAddr = "0.0.0.0"
	DefaultExporterListenPort = 5891

	// 100ms
	DefaultTimeToWaitForOpusPacketMs = 100

	// リトライ間隔 100ms
	DefaultRetryIntervalMs = 100
)

type Config struct {
	Version string

	Debug bool `ini:"debug"`

	HTTPS      bool   `ini:"https"`
	ListenAddr string `ini:"listen_addr"`
	ListenPort int    `ini:"listen_port"`

	AudioStreamingHeader bool `ini:"audio_streaming_header"`

	TLSFullchainFile    string `ini:"tls_fullchain_file"`
	TLSPrivkeyFile      string `ini:"tls_privkey_file"`
	TLSVerifyCacertPath string `ini:"tls_verify_cacert_path"` // クライアント認証用

	HTTP2MaxConcurrentStreams uint32 `ini:"http2_max_concurrent_streams"`
	HTTP2MaxReadFrameSize     uint32 `ini:"http2_max_read_frame_size"`
	HTTP2IdleTimeout          uint32 `ini:"http2_idle_timeout"`

	MaxRetry        int `ini:"max_retry"`
	RetryIntervalMs int `ini:"retry_interval_ms"`

	ExporterHTTPS      bool   `ini:"exporter_https"`
	ExporterListenAddr string `ini:"exporter_listen_addr"`
	ExporterListenPort int    `ini:"exporter_listen_port"`

	SkipBasicAuth     bool   `ini:"skip_basic_auth"`
	BasicAuthUsername string `ini:"basic_auth_username"`
	BasicAuthPassword string `ini:"basic_auth_password"`

	SampleRate   int `ini:"audio_sample_rate"`
	ChannelCount int `ini:"audio_channel_count"`

	DumpFile string `ini:"dump_file"`

	LogDir              string `ini:"log_dir"`
	LogName             string `ini:"log_name"`
	LogStdout           bool   `ini:"log_stdout"`
	LogRotateMaxSize    int    `ini:"log_rotate_max_size"`
	LogRotateMaxBackups int    `ini:"log_rotate_max_backups"`
	LogRotateMaxAge     int    `ini:"log_rotate_max_age"`

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
	AwsResultID        bool `ini:"aws_result_id"`

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

func NewConfig(configFilePath string) (*Config, error) {
	config := new(Config)

	iniConfig, err := ini.InsensitiveLoad(configFilePath)
	if err != nil {
		return nil, err
	}

	if err := iniConfig.StrictMapTo(config); err != nil {
		return nil, err
	}

	setDefaultsConfig(config)

	if err := validateConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

func setDefaultsConfig(config *Config) {
	config.Version = Version

	if config.LogDir == "" {
		config.LogDir = DefaultLogDir
	}

	if config.LogName == "" {
		config.LogName = DefaultLogName
	}

	if config.LogRotateMaxSize == 0 {
		config.LogRotateMaxSize = DefaultLogRotateMaxSize
	}

	if config.LogRotateMaxBackups == 0 {
		config.LogRotateMaxBackups = DefaultLogRotateMaxBackups
	}

	if config.LogRotateMaxAge == 0 {
		config.LogRotateMaxAge = DefaultLogRotateMaxAge
	}

	if config.ExporterListenAddr == "" {
		config.ExporterListenAddr = DefaultExporterListenAddr
	}

	if config.ExporterListenPort == 0 {
		config.ExporterListenPort = DefaultExporterListenPort
	}

	if config.TimeToWaitForOpusPacketMs == 0 {
		config.TimeToWaitForOpusPacketMs = DefaultTimeToWaitForOpusPacketMs
	}

	if config.RetryIntervalMs == 0 {
		config.RetryIntervalMs = DefaultRetryIntervalMs
	}
}

func validateConfig(config *Config) error {
	var err error
	// アドレスとして正しいことを確認する
	_, err = netip.ParseAddr(config.ListenAddr)
	if err != nil {
		return err
	}

	// アドレスとして正しいことを確認する
	_, err = netip.ParseAddr(config.ExporterListenAddr)
	if err != nil {
		return err
	}

	if config.HTTPS || config.ExporterHTTPS {
		if config.TLSFullchainFile == "" {
			return fmt.Errorf("tls_fullchain_file is required")
		}

		if config.TLSPrivkeyFile == "" {
			return fmt.Errorf("tls_privkey_file is required")
		}
	}

	return nil
}

func ShowConfig(config *Config) {

	zlog.Info().Bool("debug", config.Debug).Msg("CONF")

	zlog.Info().Str("log_dir", config.LogDir).Msg("CONF")
	zlog.Info().Str("log_name", config.LogName).Msg("CONF")
	zlog.Info().Bool("log_stdout", config.LogStdout).Msg("CONF")

	zlog.Info().Int("log_rotate_max_size", config.LogRotateMaxSize).Msg("CONF")
	zlog.Info().Int("log_rotate_max_backups", config.LogRotateMaxBackups).Msg("CONF")
	zlog.Info().Int("log_rotate_max_age", config.LogRotateMaxAge).Msg("CONF")

	zlog.Info().Bool("https", config.HTTPS).Msg("CONF")
	zlog.Info().Str("listen_addr", config.ListenAddr).Msg("CONF")
	zlog.Info().Int("listen_port", config.ListenPort).Msg("CONF")

	zlog.Info().Bool("exporter_https", config.ExporterHTTPS).Msg("CONF")
	zlog.Info().Str("exporter_listen_addr", config.ExporterListenAddr).Msg("CONF")
	zlog.Info().Int("exporter_listen_port", config.ExporterListenPort).Msg("CONF")

	zlog.Info().Int("max_retry", config.MaxRetry).Msg("CONF")
	zlog.Info().Int("retry_interval_ms", config.RetryIntervalMs).Msg("CONF")
}
