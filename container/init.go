package container

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

/*
 这里是父进程，就是当前进程执行的内容
*/ // NewParentProcess
func NewParentProcess(tty bool) (*exec.Cmd, *os.File) {
	readPipe, writePipe, err := NewPipe()
	if err != nil {
		log.Errorf("New pipe error %v", err)
		return nil, nil
	}
	// 调用自己，进行初始化 这里就是执行了init命令 所以下面就是还要看init干嘛了
	cmd := exec.Command("/proc/self/exe", "init")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS |
			syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
	}
	if tty {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	// 在这传入管道文件读取端的句柄，传给子进程
	// cmd.ExtraFiles 外带这个文件句柄去创建子进程
	cmd.ExtraFiles = []*os.File{readPipe}
	return cmd, writePipe
}

/*
  利用这个函数进行初始化，init会调用它。在容器内部执行的。也就是是，代码执行到这容器所在的进程其实已经创建出来了，就是parent。
  这是本容器执行的第一个进程。
  使用mount去挂载proc文件系统，以便后面通过ps等命令去查看当前进程资源情况
  MS_NOEXEC：在本文件系统中不允许运行其他程序
  MS_NOSUID：在本系统中运行程序时，不允许setUID或者setGID
  MS_NODEV：默认设定
*/ //RunContainerInitProcess
func RunContainerInitProcess() error {
	cmdArray := readUserCommand()
	if cmdArray == nil || len(cmdArray) == 0 {
		return fmt.Errorf("Run container get user command error, cmdArray is nil")
	}
	// systemd 加入linux之后, mount namespace 就变成 shared by default, 所以你必须显示
	// 声明你要这个新的mount namespace独立。
	err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "")
	if err != nil {
		return err
	}
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")
	// 调用exec.LookPath 可以在系统的PATH里面寻找命令的绝对路径 上一版中得写/bin/sh 现在只需要sh即可
	path, err := exec.LookPath(cmdArray[0])
	if err != nil {
		log.Errorf("Exec loop path error %v", err)
		return err
	}
	log.Infof("Find path %s", path)
	// 完成初始化，并将用户程序运行起来。这里用execve系统调用。它会覆盖当前进程的镜像、数据、堆栈等信息。PID不变。
	// 就是借原来的壳，脱胎换骨。为什么要这样。
	// 如果不这样的话，那么用户指定的命令就不是第一个进程，而是init初始化的进程。
	if err := syscall.Exec(path, cmdArray[0:], os.Environ()); err != nil {
		log.Errorf(err.Error())
	}
	return nil
}

// NewPipe 使用匿名管道来实现父子进程之间的通信
func NewPipe() (*os.File, *os.File, error) {
	read, write, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	return read, write, nil
}

// readUserCommand 子进程读取管道
func readUserCommand() []string {
	// 默认的标准IO占三个，那管道从第四个开始
	pipe := os.NewFile(uintptr(3), "pipe")
	msg, err := ioutil.ReadAll(pipe)
	if err != nil {
		log.Errorf("init read pipe error %v", err)
		return nil
	}
	msgStr := string(msg)
	return strings.Split(msgStr, " ")
}
