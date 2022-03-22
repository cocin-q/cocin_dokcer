package main

import (
	"log"
	"os"
	"os/exec"
	"syscall"
)

//
//	Go来做UTS Namespace的例子
//

func main() {
	cmd := exec.Command("sh") // 指定被fork出来的新进程内的初始命令，默认使用sh来执行
	// 设置系统调用参数 创建UTS Namespace
	cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: syscall.CLONE_NEWUTS}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}
