package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"

	zlog "github.com/rs/zerolog/log"
	"github.com/shiguredo/suzu"
	"golang.org/x/sync/errgroup"
)

func main() {
	// /bin/kohaku -V
	showVersion := flag.Bool("V", false, "バージョン")

	// bin/suzu -C config.ini
	configFilePath := flag.String("C", "./config.ini", "設定ファイルへのパス")
	serviceType := flag.String("service", "aws", fmt.Sprintf("音声文字変換のサービス（%s）", strings.Join(serviceNames(), ", ")))
	flag.Parse()

	if *showVersion {
		fmt.Printf("Audio Streaming Gateway Suzu version %s\n", suzu.Version)
		return
	}

	config, err := suzu.NewConfig(*configFilePath)
	if err != nil {
		// パースに失敗した場合 Fatal で終了
		log.Fatal("cannot parse config file, err=", err)
	}

	// ロガー初期化
	suzu.InitLogger(config)

	logger, err := suzu.NewLogger(config)
	if err != nil {
		// ロガー初期化に失敗したら Fatal で終了
		log.Fatal("cannot parse config file, err=", err)
	}
	// グローバルロガーを設定
	zlog.Logger = *logger

	suzu.ShowConfig(config)

	server, err := suzu.NewServer(config, *serviceType)
	if err != nil {
		log.Fatal("cannot create server:", err)
	}

	g, ctx := errgroup.WithContext(context.Background())

	g.Go(func() error {
		return server.Start(ctx)
	})

	g.Go(func() error {
		return server.StartExporter(ctx)
	})

	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}
}

func serviceNames() []string {
	names := suzu.NewServiceHandlerFuncs.GetNames([]string{"test", "dump"})
	sort.Strings(names)
	return names
}
