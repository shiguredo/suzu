package suzu

import (
	"github.com/BurntSushi/toml"
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

	AwsCredentialFile                    string `toml:"aws_credential_file"`
	AwsProfile                           string `toml:"aws_profile"`
	AwsRegion                            string `toml:"aws_region"`
	AwsEnablePartialResultsStabilization bool   `toml:"aws_enable_partial_results_stabilization"`
	AwsEnableChannelIdentification       bool   `toml:"aws_enable_channel_identification"`

	DumpFile string `toml:"dump_file"`

	LogDir    string `toml:"log_dir"`
	LogName   string `toml:"log_name"`
	LogDebug  bool   `toml:"log_debug"`
	LogStdout bool   `toml:"log_stdout"`

	TimeToWaitForOpusPacket string `toml:"time_to_wait_for_opus_packet"`
}

func InitConfig(data []byte, config interface{}) error {
	if err := toml.Unmarshal(data, config); err != nil {
		// パースに失敗した場合 Fatal で終了
		return err
	}

	// TODO(v): 初期値
	return nil
}
