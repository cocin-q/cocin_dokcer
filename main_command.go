package main

import (
	"cocin_dokcer/Cgroups/subsystems"
	"cocin_dokcer/container"
	"cocin_dokcer/network"
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
		cli.StringSliceFlag{
			Name:  "e",
			Usage: "set environment",
		},
		cli.StringFlag{
			Name:  "net",
			Usage: "container network",
		},
		cli.StringSliceFlag{
			Name:  "p",
			Usage: "port mapping",
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
		envSlice := context.StringSlice("e")

		network := context.String("net")
		portmapping := context.StringSlice("p")

		// imageName作为第一个参数输入
		imageName := cmdArray[0]
		cmdArray = cmdArray[1:]
		Run(tty, cmdArray, resConf, volume, containerName, imageName, envSlice, network, portmapping)
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
		if len(context.Args()) < 2 {
			return fmt.Errorf("Missing image name")
		}
		containerName := context.Args().Get(0)
		imageName := context.Args().Get(1)
		commitContainer(containerName, imageName)
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
		// 当执行这个命令的时候，设置完环境变量，会重新打开一个子进程执行exec命令，这时候父进程可退出
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

// stop命令
var stopCommand = cli.Command{
	Name:  "stop",
	Usage: "stop a container",
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("Missing container name")
		}
		containerName := context.Args().Get(0)
		stopContainer(containerName)
		return nil
	},
}

// rm命令
var removeCommand = cli.Command{
	Name:  "rm",
	Usage: "remove unused containers",
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("Missing container name")
		}
		containerName := context.Args().Get(0)
		removeContainer(containerName)
		return nil
	},
}

// network命令
var networkCommand = cli.Command{
	Name:  "network",
	Usage: "container network commands",
	Subcommands: []cli.Command{ // 子命令
		{
			Name:  "create",
			Usage: "create a container network",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "driver",
					Usage: "network driver",
				},
				cli.StringFlag{
					Name:  "subnet",
					Usage: "subnet cidr",
				},
			},
			Action: func(context *cli.Context) error {
				if len(context.Args()) < 1 {
					return fmt.Errorf("Missing network name")
				}
				network.Init()
				err := network.CreateNetwork(context.String("driver"), context.String("subnet"), context.Args()[0])
				if err != nil {
					return fmt.Errorf("create network error: %+v", err)
				}
				return nil
			},
		},
		{
			Name:  "list",
			Usage: "list container network",
			Action: func(context *cli.Context) error {
				network.Init()
				network.ListNetwork()
				return nil
			},
		},
		{
			Name:  "remove",
			Usage: "remove container network",
			Action: func(context *cli.Context) error {
				if len(context.Args()) < 1 {
					return fmt.Errorf("Missing network name")
				}
				network.Init()
				err := network.DeleteNetwork(context.Args()[0])
				if err != nil {
					return fmt.Errorf("remove network error: %+v", err)
				}
				return nil
			},
		},
	},
}
