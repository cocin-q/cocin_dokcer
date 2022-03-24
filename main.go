package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"os"
)

const usage = `cocin_docker is a simple container runtime implementation.`

func main() {
	app := cli.NewApp()
	app.Name = "cocin_docker"
	app.Usage = usage

	// 定义基本命令
	app.Commands = []cli.Command{
		initCommand,
		runCommand,
		commitCommand,
	}

	// 初始化日志配置，失败不会执行命令
	app.Before = func(context *cli.Context) error {
		log.SetFormatter(&log.JSONFormatter{})
		log.SetOutput(os.Stdout)
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
