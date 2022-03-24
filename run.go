package main

import (
	"cocin_dokcer/Cgroups"
	"cocin_dokcer/Cgroups/subsystems"
	"cocin_dokcer/container"
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
)

// Run 运行命令
func Run(tty bool, comArray []string, res *subsystems.ResourceConfig, volume string) {
	parent, writePipe := container.NewParentProcess(tty, volume)
	if parent == nil {
		log.Errorf("New parent process error")
		return
	}
	if err := parent.Start(); err != nil {
		log.Error(err)
	}
	// 创建cgroup manager
	cgroupManager := Cgroups.NewCgroupManager("cocin_docker-cgroup")
	defer cgroupManager.Destroy()
	// 设置资源限制
	cgroupManager.Set(res)
	cgroupManager.Apply(parent.Process.Pid)
	// 设置完限制后 初始化容器
	sendInitCommand(comArray, writePipe)
	parent.Wait()

	mntURL := "/root/mnt/"
	rootURL := "/root/"
	container.DeleteWorkSpace(rootURL, mntURL, volume)

	os.Exit(-1)
}

// sendInitCommand 发送用户命令进行初始化
func sendInitCommand(comArray []string, writePipe *os.File) {
	command := strings.Join(comArray, " ")
	log.Infof("command all is %s", command)
	writePipe.WriteString(command)
	writePipe.Close()
}
