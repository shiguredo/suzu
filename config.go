package suzu

import (
	"github.com/goccy/go-yaml"
)

type Config struct {
	Revision string

	Debug bool `yaml:"debug"`

	ListenAddr string `yaml:"listen_addr"`
	ListenPort int    `yaml:"listen_port"`

	HTTP2FullchainFile    string `yaml:"http2_fullchain_file"`
	HTTP2PrivkeyFile      string `yaml:"http2_privkey_file"`
	HTTP2VerifyCacertPath string `yaml:"http2_verify_cacert_path"` // クライアント認証用

	HTTP2MaxConcurrentStreams uint32 `yaml:"http2_max_concurrent_streams"`
	HTTP2MaxReadFrameSize     uint32 `yaml:"http2_max_read_frame_size"`
	HTTP2IdleTimeout          uint32 `yaml:"http2_idle_timeout"`

	ExporterIPAddress string `yaml:"exporter_ip_address"`
	ExporterPort      int    `yaml:"exporter_port"`

	SkipBasicAuth     bool   `yaml:"skip_basic_auth"`
	BasicAuthUsername string `yaml:"basic_auth_username"`
	BasicAuthPassword string `yaml:"basic_auth_password"`

	DumpFile string `yaml:"dump_file"`

	LogDir    string `yaml:"log_dir"`
	LogName   string `yaml:"log_name"`
	LogDebug  bool   `yaml:"log_debug"`
	LogStdout bool   `yaml:"log_stdout"`

	TimeToWaitForOpusPacket string `yaml:"time_to_wait_for_opus_packet"`

	// 共通
	SampleRate   int `yaml:"audio_sample_rate"`
	ChannelCount int `yaml:"audio_channel_count"`

	// amazon transcribe
	AwsCredentialFile                    string `yaml:"aws_credential_file"`
	AwsProfile                           string `yaml:"aws_profile"`
	AwsRegion                            string `yaml:"aws_region"`
	AwsEnablePartialResultsStabilization bool   `yaml:"aws_enable_partial_results_stabilization"`
	AwsEnableChannelIdentification       bool   `yaml:"aws_enable_channel_identification"`

	// google speech to text
	EnableSeparateRecognitionPerChannel bool     `yaml:"enable_separate_recognition_per_channel"`
	AlternativeLanguageCodes            []string `yaml:"alternative_language_codes"`
	MaxAlternatives                     int32    `yaml:"max_alternatives"`
	ProfanityFilter                     bool     `yaml:"profanity_filter"`
	EnableWordTimeOffsets               bool     `yaml:"enable_word_time_offsets"`
	EnableWordConfidence                bool     `yaml:"enable_word_confidence"`
	EnableAutomaticPunctuation          bool     `yaml:"enable_automatic_punctuation"`
	Model                               string   `yaml:"model"`
	UseEnhanced                         bool     `yaml:"use_enhanced"`
}

func InitConfig(data []byte, config interface{}) error {
	if err := yaml.Unmarshal(data, config); err != nil {
		// パースに失敗した場合 Fatal で終了
		return err
	}

	// TODO(v): 初期値
	return nil
}
