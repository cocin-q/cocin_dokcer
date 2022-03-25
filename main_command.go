package main

import (
	"cocin_dokcer/Cgroups/subsystems"
	"cocin_dokcer/container"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"os"
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
			Name:  "v",
			Usage: "volume",
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
		cli.BoolFlag{
			Name:  "d",
			Usage: "detach container",
		},
		cli.StringFlag{
			Name:  "name",
			Usage: "container name",
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
		var cmdArray []string
		for _, arg := range context.Args() {
			cmdArray = append(cmdArray, arg)
		}
		tty := context.Bool("ti")
		detach := context.Bool("d")
		if tty && detach { // tty 相当于前台交互模式 detach是后台运行模式
			return fmt.Errorf("ti and d paramter can not both provided")
		}
		// 把volume参数传给Run函数
		volume := context.String("v")
		resConf := &subsystems.ResourceConfig{
			MemoryLimit: context.String("mem"), // 没找到返回""
			CpuShare:    context.String("cpuset"),
			CpuSet:      context.String("cpushare"),
		}
		log.Infof("tty: %v", tty)
		containerName := context.String("name")
		Run(tty, cmdArray, resConf, volume, containerName)
		return nil
	},
}

// init命令
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

// commit命令
var commitCommand = cli.Command{
	Name:  "commit",
	Usage: "commit a container into image",
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("Missing image name")
		}
		imageName := context.Args().Get(0)
		commitContainer(imageName)
		return nil
	},
}

// ps命令
var listCommand = cli.Command{
	Name:  "ps",
	Usage: "list all the containers",
	Action: func(context *cli.Context) error {
		ListContainers()
		return nil
	},
}

// logs命令
var logCommand = cli.Command{
	Name:  "logs",
	Usage: "print logs of a container",
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("Please input your container name")
		}
		containerName := context.Args().Get(0)
		logContainer(containerName)
		return nil
	},
}

// exec命令
var execCommand = cli.Command{
	Name:  "exec",
	Usage: "exec a command into container",
	Action: func(context *cli.Context) error {
		if os.Getenv(ENV_EXEC_PID) != "" {
			log.Infof("pid callback pid %s", os.Getpid())
			return nil
		}
		// 命令格式是 cocin_docker exec 容器名 命令
		if len(context.Args()) < 2 {
			return fmt.Errorf("Missing container name or command")
		}
		containerName := context.Args().Get(0)
		var commandArray []string
		// 将除了容器名以外的参数当作需要执行的命令处理  不是返回最后一个 源码实现是返回一个切片，除容器参数外的命令切片
		for _, arg := range context.Args().Tail() {
			commandArray = append(commandArray, arg)
		}
		// 执行命令
		ExecContainer(containerName, commandArray)
		return nil
	},
}
