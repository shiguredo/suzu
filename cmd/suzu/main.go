package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/shiguredo/suzu"
	"golang.org/x/sync/errgroup"
)

var (
	// git rev-parse --short HEAD
	revision string = "air"
)

var (
	configFilePath string
	serviceType    string
)

func init() {
	// XXX(v): とりあえず 同じ場所にある config.toml を読みに行く実装
	flag.StringVar(&configFilePath, "C", "config.toml", "suzu の設定ファイルへのパス")
	flag.StringVar(&serviceType, "service", "aws", fmt.Sprintf("音声文字変換のサービス（%s）", strings.Join(suzu.NewServiceHandlerFuncs.GetNames([]string{"test", "dump"}), ", ")))
	flag.Parse()
}

func main() {

	buf, err := os.ReadFile(configFilePath)
	if err != nil {
		// 読み込めない場合 Fatal で終了
		log.Fatal("cannot open config file, err=", err)
	}

	// toml をパース
	var config suzu.Config
	if err := suzu.InitConfig(buf, &config); err != nil {
		// パースに失敗した場合 Fatal で終了
		log.Fatal("cannot parse config file, err=", err)
	}

	// リビジョンを追加
	config.Revision = revision

	// ロガー初期化
	err = suzu.InitLogger(config)
	if err != nil {
		// ロガー初期化に失敗したら Fatal で終了
		log.Fatal("cannot parse config file, err=", err)
	}

	server, err := suzu.NewServer(&config, serviceType)
	if err != nil {
		log.Fatal("cannot create server:", err)
	}

	g, ctx := errgroup.WithContext(context.Background())

	g.Go(func() error {
		return server.Start(ctx, config.ListenAddr, config.ListenPort)
	})

	g.Go(func() error {
		return server.StartExporter(ctx, config.ExporterIPAddress, config.ExporterPort)
	})

	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}
}
