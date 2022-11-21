package suzu

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo-contrib/prometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	zlog "github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
)

type Server struct {
	config       *Config
	echo         *echo.Echo
	echoExporter *echo.Echo

	http.Server
}

func NewServer(c *Config, service string) (*Server, error) {
	h2s := &http2.Server{
		MaxConcurrentStreams: c.HTTP2MaxConcurrentStreams,
		MaxReadFrameSize:     c.HTTP2MaxReadFrameSize,
		IdleTimeout:          time.Duration(c.HTTP2IdleTimeout) * time.Second,
	}

	e := echo.New()

	s := &Server{
		config: c,
		Server: http.Server{
			Addr:    net.JoinHostPort("", strconv.Itoa(c.ListenPort)),
			Handler: e,
		},
	}

	// クライアント認証をするかどうかのチェック
	if c.HTTP2VerifyCacertPath != "" {
		clientCAPath := c.HTTP2VerifyCacertPath
		certPool, err := appendCerts(clientCAPath)
		if err != nil {
			zlog.Error().Err(err).Send()
			return nil, err
		}

		tlsConfig := &tls.Config{
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  certPool,
		}
		s.Server.TLSConfig = tlsConfig
	}

	if err := http2.ConfigureServer(&s.Server, h2s); err != nil {
		return nil, err
	}

	e.Pre(middleware.RemoveTrailingSlash())

	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Skipper: func(c echo.Context) bool {
			// /health の時はログを吐き出さない
			return strings.HasPrefix(c.Request().URL.Path, "/.ok")
		},
	}))

	e.Use(middleware.Recover())

	// LB からのヘルスチェック専用 API
	e.GET("/.ok", s.healthcheckHandler)

	switch service {
	case "aws":
		e.POST("/speech", s.createSpeechHandler(AmazonTranscribeHandler))
	case "gcp":
		e.POST("/speech", s.createSpeechHandler(SpeechToTextHandler))
	default:
		return nil, fmt.Errorf("UNEXPECTED-SERVICE")
	}
	e.POST("/test", s.createSpeechHandler(TestHandler))
	e.POST("/dump", s.createSpeechHandler(PacketDumpHandler))

	echoExporter := echo.New()
	echoExporter.HideBanner = true
	prom := prometheus.NewPrometheus("echo", nil)

	e.Use(prom.HandlerFunc)
	prom.SetMetricsPath(echoExporter)

	s.echo = e
	s.echoExporter = echoExporter

	return s, nil
}

func (s *Server) Start(ctx context.Context, address string, port int) error {
	http2FullchainFile := s.config.HTTP2FullchainFile
	http2PrivkeyFile := s.config.HTTP2PrivkeyFile

	if _, err := os.Stat(http2FullchainFile); err != nil {
		return fmt.Errorf("http2FullchainFile error: %s", err)
	}

	if _, err := os.Stat(http2PrivkeyFile); err != nil {
		return fmt.Errorf("http2PrivkeyFile error: %s", err)
	}

	ch := make(chan error)
	go func() {
		defer close(ch)
		if err := s.ListenAndServeTLS(http2FullchainFile, http2PrivkeyFile); err != http.ErrServerClosed {
			ch <- err
			if err := s.Shutdown(ctx); err != nil {
				// TODO: ch は待ち受けていないので、ここでログ出力
			}
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		return err
	}

}

func (s *Server) StartExporter(ctx context.Context, address string, port int) error {
	ch := make(chan error)
	go func() {
		err := s.echoExporter.Start(net.JoinHostPort(address, strconv.Itoa(port)))
		if err != nil {
			ch <- err
			if err := s.echoExporter.Shutdown(ctx); err != nil {
				// TODO: ch は待ち受けていないので、ここでログ出力
			}
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		return err
	}
}

func appendCerts(clientCAPath string) (*x509.CertPool, error) {
	certPool := x509.NewCertPool()
	fi, err := os.Stat(clientCAPath)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		files, err := os.ReadDir(clientCAPath)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			clientCAPath := filepath.Join(clientCAPath, f.Name())
			if err := appendCertsFromPEM(certPool, clientCAPath); err != nil {
				return nil, err
			}
		}
	} else {
		if err := appendCertsFromPEM(certPool, clientCAPath); err != nil {
			return nil, err
		}
	}
	return certPool, nil
}

func appendCertsFromPEM(certPool *x509.CertPool, filepath string) error {
	clientCA, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}
	ok := certPool.AppendCertsFromPEM(clientCA)
	if !ok {
		return fmt.Errorf("failed to append certificates: %s", filepath)
	}
	return nil
}
