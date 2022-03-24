package practice

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"syscall"
)

// 挂载了memory subsystem的hierarchy的根目录位置
const cgroupMemoryHierarchy = "/sys/fs/cgroup/memory"

func cgroup() {
	//fmt.Println(os.Args)
	if os.Args[0] == "/proc/self/exe" {
		// 容器进程
		fmt.Printf("current pid %d\n", syscall.Getpid())
		cmd := exec.Command("sh", "-c", `stress --vm-bytes 200m --vm-keep -m 1`)
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		// Run会阻塞等待命令完成
		if err := cmd.Run(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
	// /proc/self/exe 它代表当前程序，即运行当前程序
	cmd := exec.Command("/proc/self/exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: syscall.CLONE_NEWUTS |
		syscall.CLONE_NEWPID | syscall.CLONE_NEWNS}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Start不会等待命令完成，这里相当于创建了一个子进程，上面的if判断需要用到这个子进程
	if err := cmd.Start(); err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	} else {
		// 得到fork出来的进程映射在外部命名空间的PID
		fmt.Printf("%v\n", cmd.Process.Pid)
		// 在系统默认创建挂载了 memory subsystem的Hierarchy上创建cgroup
		os.Mkdir(path.Join(cgroupMemoryHierarchy, "testmemorylimit"), 0755)
		// 将容器进程加入到这个cgroup中
		ioutil.WriteFile(path.Join(cgroupMemoryHierarchy, "testmemorylimit", "tasks"), []byte(strconv.Itoa(cmd.Process.Pid)), 0644)
		// 限制cgroup进程使用
		ioutil.WriteFile(path.Join(cgroupMemoryHierarchy, "testmemorylimit", "memory.limit_in_bytes"), []byte("100m"), 0644)
		cmd.Process.Wait()
	}
}
