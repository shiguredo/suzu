package main

import (
	"flag"
	"log"
	"os"

	"github.com/shiguredo/suzu"
	"golang.org/x/sync/errgroup"
)

var (
	// git rev-parse --short HEAD
	revision string = "air"
)

var (
	g errgroup.Group
)

func main() {
	// XXX(v): とりあえず 同じ場所にある config.yaml を読みに行く実装
	configFilePath := flag.String("C", "config.yaml", "Tobi の設定ファイルへのパス")
	flag.Parse()

	buf, err := os.ReadFile(*configFilePath)
	if err != nil {
		// 読み込めない場合 Fatal で終了
		log.Fatal("cannot open config file, err=", err)
	}

	// yaml をパース
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

	server := suzu.NewServer(&config)
	if err != nil {
		log.Fatal("cannot create server:", err)
	}

	g.Go(func() error {
		return server.Start(config.ListenAddr, config.ListenPort)
	})

	g.Go(func() error {
		return server.StartExporter(config.ExporterIPAddress, config.ExporterPort)
	})

	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}
}
