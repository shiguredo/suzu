package suzu

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/shiguredo/lumberjack/v3"
)

const (
	// megabytes
	DefaultLogRotateMaxSize    = 200
	DefaultLogRotateMaxBackups = 7
	// days
	DefaultLogRotateMaxAge = 30
)

// InitLogger ロガーを初期化する
func InitLogger(config Config) error {
	if f, err := os.Stat(config.LogDir); os.IsNotExist(err) || !f.IsDir() {
		return err
	}

	logPath := fmt.Sprintf("%s/%s", config.LogDir, config.LogName)

	// https://github.com/rs/zerolog/issues/77
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().UTC()
	}

	zerolog.TimeFieldFormat = time.RFC3339Nano

	if config.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	if config.Debug && config.LogStdout {
		writer := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "2006-01-02 15:04:05.000000Z07:00:00"}
		format(&writer)
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

func format(w *zerolog.ConsoleWriter) {
	w.FormatLevel = func(i interface{}) string {
		return strings.ToUpper(fmt.Sprintf("[%s]", i))
	}
	w.FormatFieldName = func(i interface{}) string {
		return fmt.Sprintf("%s=", i)
	}
	w.FormatFieldValue = func(i interface{}) string {
		return fmt.Sprintf("%s", i)
	}
}
