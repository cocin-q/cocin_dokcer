package container

import (
	"github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"syscall"
)

/*
 这里是父进程，就是当前进程执行的内容
*/ // NewParentProcess
func NewParentProcess(tty bool, command string) *exec.Cmd {
	args := []string{"init", command}
	// 调用自己，进行初始化 这里就是执行了init命令 所以下面就是还要看init干嘛了
	cmd := exec.Command("/proc/self/exe", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS |
			syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
	}
	if tty {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd
}

/*
  利用这个函数进行初始化，init会调用它。在容器内部执行的。也就是是，代码执行到这容器所在的进程其实已经创建出来了，就是parent。
  这是本容器执行的第一个进程。
  使用mount去挂载proc文件系统，以便后面通过ps等命令去查看当前进程资源情况
  MS_NOEXEC：在本文件系统中不允许运行其他程序
  MS_NOSUID：在本系统中运行程序时，不允许setUID或者setGID
  MS_NODEV：默认设定
*/ //RunContainerInitProcess
func RunContainerInitProcess(command string, args []string) error {
	logrus.Infof("command %s", command)
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")
	argv := []string{command}
	// 完成初始化，并将用户程序运行起来。这里用execve系统调用。它会覆盖当前进程的镜像、数据、堆栈等信息。PID不变。
	// 就是借原来的壳，脱胎换骨。为什么要这样。
	// 如果不这样的话，那么用户指定的命令就不是第一个进程，而是init初始化的进程。
	if err := syscall.Exec(command, argv, os.Environ()); err != nil {
		logrus.Errorf(err.Error())
	}
	return nil
}
