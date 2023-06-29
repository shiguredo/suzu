package suzu

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/netip"
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

	_, err := netip.ParseAddr(c.ListenAddr)
	if err != nil {
		return nil, err
	}

	e := echo.New()

	s := &Server{
		config: c,
		Server: http.Server{
			Addr:    net.JoinHostPort(c.ListenAddr, strconv.Itoa(c.ListenPort)),
			Handler: e,
		},
	}

	// クライアント認証をするかどうかのチェック
	if c.TLSVerifyCacertPath != "" {
		clientCAPath := c.TLSVerifyCacertPath
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

	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		Skipper: func(c echo.Context) bool {
			// /health の時はログを吐き出さない
			return strings.HasPrefix(c.Request().URL.Path, "/.ok")
		},
		LogRemoteIP:      true,
		LogHost:          true,
		LogMethod:        true,
		LogURI:           true,
		LogStatus:        true,
		LogError:         true,
		LogLatency:       true,
		LogUserAgent:     true,
		LogContentLength: true,
		LogResponseSize:  true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			zlog.Info().
				Str("remote_ip", v.RemoteIP).
				Str("host", v.Host).
				Str("user_agent", v.UserAgent).
				Str("uri", v.URI).
				Int("status", v.Status).
				Err(v.Error).
				Str("latency", v.Latency.String()).
				Str("bytes_in", v.ContentLength).
				Int64("bytes_out", v.ResponseSize).
				Msg(v.Method)

			return nil
		},
	}))

	e.Use(middleware.Recover())

	// LB からのヘルスチェック専用 API
	e.GET("/.ok", s.healthcheckHandler)

	e.POST("/speech", s.createSpeechHandler(service, nil), HTTPVersionValidation, s.SuzuContext)
	e.POST("/test", s.createSpeechHandler("test", nil), HTTPVersionValidation, s.SuzuContext)
	e.POST("/dump", s.createSpeechHandler("dump", nil), HTTPVersionValidation, s.SuzuContext)

	echoExporter := echo.New()
	echoExporter.HideBanner = true
	echoExporter.HidePort = true
	prom := prometheus.NewPrometheus("echo", nil)

	e.Use(prom.HandlerFunc)
	prom.SetMetricsPath(echoExporter)

	s.echo = e
	s.echoExporter = echoExporter

	zlog.Info().Str("service_type", service).Send()

	return s, nil
}

func (s *Server) Start(ctx context.Context, address string, port int) error {
	tlsFullchainFile := s.config.TLSFullchainFile
	tlsPrivkeyFile := s.config.TLSPrivkeyFile

	if _, err := os.Stat(tlsFullchainFile); err != nil {
		return fmt.Errorf("tlsFullchainFile error: %s", err)
	}

	if _, err := os.Stat(tlsPrivkeyFile); err != nil {
		return fmt.Errorf("tls2PrivkeyFile error: %s", err)
	}

	ch := make(chan error)
	go func() {
		defer close(ch)
		if err := s.ListenAndServeTLS(tlsFullchainFile, tlsPrivkeyFile); err != http.ErrServerClosed {
			ch <- err
		}
	}()

	defer func() {
		if err := s.Shutdown(ctx); err != nil {
			zlog.Error().Err(err).Send()
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
		}
	}()

	defer func() {
		if err := s.echoExporter.Shutdown(ctx); err != nil {
			zlog.Error().Err(err).Send()
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
