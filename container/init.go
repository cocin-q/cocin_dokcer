package container

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

/*
 这里是父进程，就是当前进程执行的内容
*/ // NewParentProcess
func NewParentProcess(tty bool, volume, containerName, imageName string, envSlice []string) (*exec.Cmd, *os.File) {
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
	// 设置环境变量
	cmd.Env = append(os.Environ(), envSlice...)
	if tty {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		// 生成容器对应目录container.log
		dirURL := fmt.Sprintf(DefaultInfoLocation, containerName)
		if err := os.MkdirAll(dirURL, 0622); err != nil {
			log.Errorf("NewParentProcess mkdir %s error %v", dirURL, err)
			return nil, nil
		}
		stdLogFilePath := dirURL + ContainerLogFile
		stdLogFile, err := os.Create(stdLogFilePath)
		if err != nil {
			log.Errorf("NewParentProcess create file %s error %v", stdLogFilePath, err)
			return nil, nil
		}
		// 重定向
		cmd.Stdout = stdLogFile
	}
	NewWorkSpace(volume, imageName, containerName)
	cmd.Dir = fmt.Sprintf(MntUrl, containerName)
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
	setUpMount()
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

/*
 pivot_root是一个系统调用，主要是去改变当前的root文件系统。piovt_root可以将当前进程的root文件系统移动到put_old文件夹中，
 然后使new_root成为新的root文件系统。pivot_root是把整个系统切换到一个新的root目录，而移除对之前root文件系统的依赖，这样就能umount原先的root文件系统
*/
func pivotRoot(root string) error {
	/*
		为了使当前root的老root和新root不在同一个文件系统下，我们把root重新mount了一次，
		bind mount是把相同的内容换了一个挂载点的挂载方法。
		我们可以通过mount --bind命令来将两个目录连接起来，
		mount --bind命令是将前一个目录挂载到后一个目录上，所有对后一个目录的访问其实都是对前一个目录的访问。
		为什么要这样做？因为new_root必须是mount point。
	*/
	if err := syscall.Mount(root, root, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("Mount rootfs to itself error: %v", err)
	}
	// 创建 rootfs/.pivot_root 存储old_root
	// put_old必须是new_root，或者new_root的子目录，在这创建一个子目录
	pivotDir := filepath.Join(root, ".pivot_root")
	if err := os.Mkdir(pivotDir, 0777); err != nil {
		return err
	}
	//将父root设为private
	// systemd 加入linux之后, mount namespace 就变成 shared by default, 所以你必须显示
	// 声明你要这个新的mount namespace独立。
	err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "")
	if err != nil {
		return err
	}
	// pivot_root 到新的rootfs，老的old_root现在挂载在rootfs/.pivot_root上
	// pivot_root改变当前进程所在mount namespace内的所有进程的root mount移到put_old，然后将new_root作为新的root mount；
	// 挂载点目前依然可以在mount命令中看到
	if err := syscall.PivotRoot(root, pivotDir); err != nil {
		return fmt.Errorf("pivot_root %v", err)
	}
	// 修改当前的工作目录到根目录
	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / %v", err)
	}

	pivotDir = filepath.Join("/", ".pivot_root")
	// umount rootfs/.pivot_root 把原先那些根目录取消挂载了，实现隔离
	// 如果函数执行带有此参数，不会立即执行umount操作，而会等挂载点退出忙碌状态时才会去卸载它。
	// 不过此函数执行会阻止对该挂载点执行新的访问。之前就在访问此挂载点操作也不会强制其退出，而是会等待其自然退出。
	if err := syscall.Unmount(pivotDir, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_root dor %v", err)
	}
	// 删除临时文件夹
	return os.Remove(pivotDir)
}

/*
 init挂载点
*/
func setUpMount() {
	// 获取当前路径
	pwd, err := os.Getwd()
	if err != nil {
		log.Errorf("Get current location error %v", err)
		return
	}
	log.Infof("Current location is %s", pwd)
	pivotRoot(pwd)

	// mount proc
	// 因为上面pivotRoot已经把mount Namespace设置成私有不共享的了，这里不需要再设置
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")

	// 挂载虚存
	// tmpfs是Linux/Unix系统上的一种基于内存的文件系统。tmpfs可以使用RAM或swap分区来存储文件。由此可见，temfs主要存储暂存的文件。
	// 临时性、快速读写能力、动态收缩
	syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755")
}
