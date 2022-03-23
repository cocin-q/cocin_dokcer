package Namespace

import (
	"log"
	"os"
	"os/exec"
	"syscall"
)

//
//	Go来做UTS Namespace的例子
//

func namespace() {
	cmd := exec.Command("sh") // 指定被fork出来的新进程内的初始命令，默认使用sh来执行
	// 设置系统调用参数 创建UTS Namespace
	cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: syscall.CLONE_NEWUTS |
		syscall.CLONE_NEWIPC | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS |
		syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET,
		UidMappings: []syscall.SysProcIDMap{ // 设置当前空间的UID和GID，和之前不一样
			{
				ContainerID: 1234,
				HostID:      syscall.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 1234,
				HostID:      syscall.Getgid(),
				Size:        1,
			},
		},
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// 设置当前空间的UID和GID
	//cmd.SysProcAttr.Credential = &syscall.Credential{
	//	Uid: uint32(1),
	//	Gid: uint32(1),
	//}
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
	os.Exit(-1)
}
