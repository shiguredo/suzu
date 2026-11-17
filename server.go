package suzu

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	zlog "github.com/rs/zerolog/log"
)

// Server は Suzu の HTTP サーバー
type Server struct {
	// 起動時に読み込んだ設定
	config *Config
	// 音声ストリーミングを受け付けるメインサーバー
	mainServer *http.Server
	// Prometheus のメトリクスを公開する Exporter
	exporterServer *http.Server
}

// NewServer はメインサーバーと Exporter を組み立てる
// /speech には service で指定したサービスのハンドラーを登録する
func NewServer(c *Config, service string) (*Server, error) {
	_, err := netip.ParseAddr(c.ListenAddr)
	if err != nil {
		return nil, err
	}

	s := &Server{
		config: c,
	}

	mux := http.NewServeMux()

	// disable_options_response が有効でなければ、登録したパスへの OPTIONS にも応答する
	respondToOptions := !c.DisableOptionsResponse

	// LB からのヘルスチェック専用 API
	handleRoute(mux, http.MethodGet, "/.ok", errorHandlerFunc(s.healthcheckHandler), respondToOptions)

	handleRoute(mux, http.MethodPost, "/speech", s.createSpeechHandler(service, nil), respondToOptions)
	handleRoute(mux, http.MethodPost, "/test", s.createSpeechHandler("test", nil), respondToOptions)
	handleRoute(mux, http.MethodPost, "/dump", s.createSpeechHandler("dump", nil), respondToOptions)

	// HTTP/1.1、TLS 上の HTTP/2、HTTP/2 over TCP (h2c) を受け付ける
	// /.ok は HTTP/1.1 のヘルスチェックにも応答するため、HTTP/1.1 も受け付ける
	// HTTP/2 必須の /speech などは、HTTP/1.1 のリクエストをハンドラーで 400 にする
	// ErrorLog は設定しないため、TLS ハンドシェイクの失敗などの内部エラーは Go 標準の log に出力される (Exporter も同じ)
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(true)
	protocols.SetUnencryptedHTTP2(true)

	s.mainServer = &http.Server{
		Addr:      net.JoinHostPort(c.ListenAddr, strconv.Itoa(c.ListenPort)),
		Handler:   newMainHandler(mux, !c.DisableTrailingSlashRemoval),
		Protocols: protocols,
		HTTP2: &http.HTTP2Config{
			MaxConcurrentStreams: int(c.HTTP2MaxConcurrentStreams),
			MaxReadFrameSize:     int(c.HTTP2MaxReadFrameSize),
		},
		IdleTimeout: time.Duration(c.HTTP2IdleTimeout) * time.Second,
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
		s.mainServer.TLSConfig = tlsConfig
	}

	// Exporter は Prometheus からのスクレイプ専用のため、クライアント認証、アクセスログ、メトリクスの計測を行わない
	exporterMux := http.NewServeMux()
	// promhttp.Handler は既定のレジストリを公開するため、パッケージの初期化時に登録した HTTP メトリクスと、
	// go_* / process_* / promhttp_metric_handler_* が出力される
	handleRoute(exporterMux, http.MethodGet, "/metrics", promhttp.Handler(), respondToOptions)

	s.exporterServer = &http.Server{
		Addr:    net.JoinHostPort(c.ExporterListenAddr, strconv.Itoa(c.ExporterListenPort)),
		Handler: exporterMux,
	}

	zlog.Info().Str("service_type", service).Send()

	return s, nil
}

// Start はメインサーバーで待ち受ける
// ctx が終了したら Shutdown を呼んで ctx.Err() を返し、待ち受けに失敗した場合はそのエラーを返す
func (s *Server) Start(ctx context.Context) error {
	// ctx の終了後に待ち受けがエラーで終了した場合も送信で止まらないように、バッファを 1 つ持たせる
	ch := make(chan error, 1)
	go func() {
		defer close(ch)
		if s.config.HTTPS {
			if err := s.mainServer.ListenAndServeTLS(s.config.TLSFullchainFile, s.config.TLSPrivkeyFile); !errors.Is(err, http.ErrServerClosed) {
				ch <- err
			}
		} else {
			// TLS を使わない場合、HTTP/2 は HTTP/2 over TCP (h2c) になる
			if err := s.mainServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				ch <- err
			}
		}
	}()

	defer func() {
		if err := s.mainServer.Shutdown(ctx); err != nil {
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

// StartExporter は Exporter で待ち受ける
// ctx の終了時と待ち受けの失敗時の挙動は Start と同じ
func (s *Server) StartExporter(ctx context.Context) error {
	// ctx の終了後に待ち受けがエラーで終了した場合も送信で止まらないように、バッファを 1 つ持たせる
	ch := make(chan error, 1)
	go func() {
		defer close(ch)
		if s.config.ExporterHTTPS {
			if err := s.exporterServer.ListenAndServeTLS(s.config.TLSFullchainFile, s.config.TLSPrivkeyFile); !errors.Is(err, http.ErrServerClosed) {
				ch <- err
			}
		} else {
			if err := s.exporterServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				ch <- err
			}
		}
	}()

	defer func() {
		if err := s.exporterServer.Shutdown(ctx); err != nil {
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

// handleRoute は method と path のルートに h を登録する
// respondToOptions が true の場合は、同じ path への OPTIONS に応答するルートも登録する
// OPTIONS には、受け付けるメソッドを Allow ヘッダーに入れて 204 を返す
// OPTIONS のルートがあると、メソッドが合わない 405 の Allow ヘッダーにも ServeMux が OPTIONS を含める
// OPTIONS のルートを path ごとに 1 つ登録するため、同じ path に 2 つ目のメソッドを登録すると ServeMux が重複として panic する
func handleRoute(mux *http.ServeMux, method, path string, h http.Handler, respondToOptions bool) {
	mux.Handle(method+" "+path, h)
	if !respondToOptions {
		return
	}

	// ServeMux の 405 の Allow ヘッダーと同じく、メソッドを名前の順に並べる
	// ServeMux は GET のルートを HEAD にも一致させるため、GET の場合は HEAD も含める
	allow := []string{method, http.MethodOptions}
	if method == http.MethodGet {
		allow = append(allow, http.MethodHead)
	}
	slices.Sort(allow)
	allowHeader := strings.Join(allow, ", ")

	mux.Handle(http.MethodOptions+" "+path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", allowHeader)
		w.WriteHeader(http.StatusNoContent)
	}))
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
