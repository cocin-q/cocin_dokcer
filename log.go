package main

import (
	"cocin_dokcer/container"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
)

func logContainer(containerName string) {
	// 找到对应文件夹的位置
	dirURL := fmt.Sprintf(container.DefaultInfoLocation, containerName)
	logFileLocation := dirURL + container.ContainerLogFile
	// 打开日志文件
	file, err := os.Open(logFileLocation)
	defer file.Close()
	if err != nil {
		log.Errorf("Log container open file %s error %v", logFileLocation, err)
		return
	}
	// 将文件内的内容都读取出来
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Errorf("Log container read file %s error %v", logFileLocation, err)
		return
	}
	// 读取出来的内容重定向到标准输出
	fmt.Fprint(os.Stdout, string(content))
}
