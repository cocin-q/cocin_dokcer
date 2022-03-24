package main

import (
	"cocin_dokcer/Cgroups/subsystems"
	"cocin_dokcer/container"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// run命令
var runCommand = cli.Command{
	Name:  "run",
	Usage: "Create a container with namespace and cgroups limit cocin_docker run -ti [command]",
	Flags: []cli.Flag{ // 类似运行命令时使用 -- 来指定参数
		cli.BoolFlag{
			Name:  "ti",
			Usage: "enable try",
		},
		cli.StringFlag{
			Name:  "mem",
			Usage: "memory limit",
		},
		cli.StringFlag{
			Name:  "cpushare",
			Usage: "cpushare limit",
		},
		cli.StringFlag{
			Name:  "cpuset",
			Usage: "cpuset limit",
		},
	},
	/* 这里是run命令执行的真正函数
	1. 判断参数是否包含command
	2. 获取用户指定的command
	3. 调用Run function 去准备启动容器
	*/
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("Missing container command")
		}
		tty := context.Bool("ti")
		resConf := &subsystems.ResourceConfig{
			MemoryLimit: context.String("mem"), // 没找到返回""
			CpuShare:    context.String("cpuset"),
			CpuSet:      context.String("cpushare"),
		}
		//log.Infof("test: %v", resConf.MemoryLimit)
		var cmdArray []string
		for _, arg := range context.Args() {
			cmdArray = append(cmdArray, arg)
		}
		Run(tty, cmdArray, resConf)
		return nil
	},
}

var initCommand = cli.Command{
	Name:  "init",
	Usage: "Init container process run user's process in container. Do not call it outside",
	/*
		1. 获取传递过来的command参数
		2. 执行容器初始化操作
	*/
	Action: func(context *cli.Context) error {
		log.Infof("init come on")
		err := container.RunContainerInitProcess()
		return err
	},
}
