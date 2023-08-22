package suzu

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

// InitLogger ロガーを初期化する
func InitLogger(config *Config) error {
	if f, err := os.Stat(config.LogDir); os.IsNotExist(err) || !f.IsDir() {
		return err
	}

	logPath := fmt.Sprintf("%s/%s", config.LogDir, config.LogName)

	// https://github.com/rs/zerolog/issues/77
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().UTC()
	}

	zerolog.TimeFieldFormat = "2006-01-02T15:04:05.000000Z"

	if config.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	if config.Debug && config.LogStdout {
		writer := zerolog.ConsoleWriter{
			Out: os.Stdout,
			FormatTimestamp: func(i interface{}) string {
				darkGray := "\x1b[90m"
				reset := "\x1b[0m"
				return strings.Join([]string{darkGray, i.(string), reset}, "")
			},
			NoColor: false,
		}
		prettyFormat(&writer)
		log.Logger = zerolog.New(writer).With().Caller().Timestamp().Logger()
	} else if config.LogStdout {
		writer := os.Stdout
		log.Logger = zerolog.New(writer).With().Caller().Timestamp().Logger()
	} else {
		var logRotateMaxSize, logRotateMaxBackups, logRotateMaxAge int
		if config.LogRotateMaxSize == 0 {
			logRotateMaxSize = DefaultLogRotateMaxSize
		}
		if config.LogRotateMaxBackups == 0 {
			logRotateMaxBackups = DefaultLogRotateMaxBackups
		}
		if config.LogRotateMaxAge == 0 {
			logRotateMaxAge = DefaultLogRotateMaxAge
		}

		writer := &lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    logRotateMaxSize,
			MaxBackups: logRotateMaxBackups,
			MaxAge:     logRotateMaxAge,
			Compress:   false,
		}
		log.Logger = zerolog.New(writer).With().Caller().Timestamp().Logger()
	}

	return nil
}

// 現時点での prettyFormat
// 2023-04-17 12:51:56.333485Z [INFO] config.go:102 > CONF | debug=true
func prettyFormat(w *zerolog.ConsoleWriter) {
	const Reset = "\x1b[0m"

	w.FormatLevel = func(i interface{}) string {
		var color, level string
		// TODO: 各色を定数に置き換える
		// TODO: 他の logLevel が必要な場合は追加する
		switch i.(string) {
		case "info":
			color = "\x1b[32m"
		case "error":
			color = "\x1b[31m"
		case "warn":
			color = "\x1b[33m"
		case "debug":
			color = "\x1b[34m"
		default:
			color = "\x1b[37m"
		}

		level = strings.ToUpper(i.(string))
		return fmt.Sprintf("%s[%s]%s", color, level, Reset)
	}
	w.FormatCaller = func(i interface{}) string {
		return fmt.Sprintf("[%s]", filepath.Base(i.(string)))
	}
	// TODO: Caller をファイル名と行番号だけの表示で出力する
	//       以下のようなフォーマットにしたい
	//       2023-04-17 12:50:09.334758Z [INFO] [config.go:102] CONF | debug=true
	// TODO: name=value が無い場合に | を消す方法がわからなかった
	w.FormatMessage = func(i interface{}) string {
		if i == nil {
			return ""
		} else {
			return fmt.Sprintf("%s |", i)
		}
	}
	w.FormatFieldName = func(i interface{}) string {
		const Cyan = "\x1b[36m"
		return fmt.Sprintf("%s%s=%s", Cyan, i, Reset)
	}
	// TODO: カンマ区切りを同実現するかわからなかった
	w.FormatFieldValue = func(i interface{}) string {
		return fmt.Sprintf("%s", i)
	}
}
