package main

import (
	"cocin_dokcer/container"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"text/tabwriter"
)

// ListContainers 列出容器信息
func ListContainers() {
	// 找到存储信息的路径 /var/run/cocin_docker
	dirURL := fmt.Sprintf(container.DefaultInfoLocation, "")
	// "/var/run/cocin_docker/%s/" 需要把后面那个 / 去掉
	dirURL = dirURL[:len(dirURL)-1]
	// 读取该文件夹下面所有文件
	files, err := ioutil.ReadDir(dirURL)
	if err != nil {
		log.Errorf("Read dir %s error %v", dirURL, err)
		return
	}
	var containers []*container.ContainerInfo
	// 遍历文件夹下面的所有文件
	for _, file := range files {
		if file.Name() == "network" {
			continue
		}
		// 根据容器配置文件获得对应信息，然后转换成容器信息的对象
		tmpContainer, err := getContainerInfo(file)
		if err != nil {
			log.Errorf("Get container info error %v", err)
			continue
		}
		containers = append(containers, tmpContainer)
	}

	// 使用tabwriter.NewWriter 在控制台打印容器信息
	// tabwriter 是引用的 text/tabwriter 类库，用于在控制台打印对齐的表格
	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	fmt.Fprint(w, "ID\tNAME\tPID\tSTATUS\tCOMMAND\tCREATED\n")
	for _, item := range containers {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			item.Id,
			item.Name,
			item.Pid,
			item.Status,
			item.Command,
			item.CreatedTime)
	}
	if err := w.Flush(); err != nil {
		log.Errorf("Flush error %v", err)
		return
	}
}

// 因为具体的容器信息在对应容器文件夹下，所以这里需要进入文件夹取出信息
func getContainerInfo(file os.FileInfo) (*container.ContainerInfo, error) {
	// 获取文件名
	containerName := file.Name()
	// 根据文件名生成文件绝对路径
	configFileDir := fmt.Sprintf(container.DefaultInfoLocation, containerName)
	configFileDir = configFileDir + container.ConfigName
	// 读取信息
	content, err := ioutil.ReadFile(configFileDir)
	if err != nil {
		log.Errorf("Read file %s error %v", configFileDir, err)
		return nil, err
	}
	// json反序列化
	var containerInfo container.ContainerInfo
	if err := json.Unmarshal(content, &containerInfo); err != nil {
		log.Errorf("Json unmarshal error %v", err)
		return nil, err
	}
	return &containerInfo, nil
}
