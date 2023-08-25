package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/shiguredo/suzu"
	"golang.org/x/sync/errgroup"
)

var (
	// git rev-parse --short HEAD
	revision string = "air"
)

func main() {
	// XXX(v): とりあえず 同じ場所にある config.ini を読みに行く実装
	configFilePath := flag.String("C", "config.ini", "suzu の設定ファイルへのパス")
	serviceType := flag.String("service", "aws", fmt.Sprintf("音声文字変換のサービス（%s）", strings.Join(suzu.NewServiceHandlerFuncs.GetNames([]string{"test", "dump"}), ", ")))
	flag.Parse()

	config, err := suzu.NewConfig(*configFilePath)
	if err != nil {
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
