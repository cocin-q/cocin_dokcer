package main

import (
	"cocin_dokcer/Cgroups"
	"cocin_dokcer/Cgroups/subsystems"
	"cocin_dokcer/container"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

const containerIDLength = 10

// Run 运行命令
func Run(tty bool, comArray []string, res *subsystems.ResourceConfig, volume, containerName, imageName string, envSlice []string) {
	// 生成ID
	id := randStringBytes(containerIDLength)
	// 没指定名字，按照ID来
	if containerName == "" {
		containerName = id
	}

	parent, writePipe := container.NewParentProcess(tty, volume, containerName, imageName, envSlice)
	if parent == nil {
		log.Errorf("New parent process error")
		return
	}
	if err := parent.Start(); err != nil {
		log.Error(err)
	}
	// 记录容器信息
	containerName, err := recordContainerInfo(parent.Process.Pid, comArray, containerName, id, volume)
	if err != nil {
		log.Errorf("Record container info error %v", err)
		return
	}

	// 创建cgroup manager
	cgroupManager := Cgroups.NewCgroupManager("cocin_docker-cgroup")
	defer cgroupManager.Destroy()
	// 设置资源限制
	cgroupManager.Set(res)
	cgroupManager.Apply(parent.Process.Pid)
	// 设置完限制后 初始化容器
	sendInitCommand(comArray, writePipe)
	if tty {
		parent.Wait()
		deleteContainerInfo(containerName)
		container.DeleteWorkSpace(volume, containerName)
	}

	os.Exit(-1)
}

// sendInitCommand 发送用户命令进行初始化
func sendInitCommand(comArray []string, writePipe *os.File) {
	command := strings.Join(comArray, " ")
	log.Infof("command all is %s", command)
	writePipe.WriteString(command)
	writePipe.Close()
}

// ID 生成器
func randStringBytes(n int) string {
	letterBytes := "1234567890"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

// 记录容器的基本信息
func recordContainerInfo(containerPID int, commandArray []string, containerName, id, volume string) (string, error) {
	// 当前时间作为创建时间
	createTime := time.Now().Format("2006-01-02 15:04:05")
	command := strings.Join(commandArray, "")
	// 生成容器信息结构体
	containerInfo := &container.ContainerInfo{
		Pid:         strconv.Itoa(containerPID),
		Id:          id,
		Name:        containerName,
		Command:     command,
		CreatedTime: createTime,
		Status:      container.RUNNING,
		Volume:      volume,
	}

	// json序列化
	jsonBytes, err := json.Marshal(containerInfo)
	if err != nil {
		log.Errorf("Record container info error %v", err)
		return "", err
	}
	jsonStr := string(jsonBytes)

	// 拼凑存储容器信息的路径
	dirUrl := fmt.Sprintf(container.DefaultInfoLocation, containerName)
	// 路径不存在，级联的创建 如果目录已经存在，也返回nil
	if err := os.MkdirAll(dirUrl, 0622); err != nil {
		log.Errorf("Mkdir error %s error %v", dirUrl, err)
		return "", err
	}
	fileName := dirUrl + "/" + container.ConfigName
	// 创建最终配置文件 如果文件已存在，会将文件清空
	file, err := os.Create(fileName)
	defer file.Close()
	if err != nil {
		log.Errorf("Create file %s error %v", fileName, err)
		return "", err
	}
	// 将json序列化后的数据写入文件
	if _, err := file.WriteString(jsonStr); err != nil {
		log.Errorf("File write string error %v", err)
		return "", err
	}
	return containerName, nil
}

func deleteContainerInfo(containerId string) {
	dirURL := fmt.Sprintf(container.DefaultInfoLocation, containerId)
	if err := os.RemoveAll(dirURL); err != nil {
		log.Errorf("Remove dir %s error %v", dirURL, err)
	}
}
