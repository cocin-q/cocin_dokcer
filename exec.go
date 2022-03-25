package main

import (
	"cocin_dokcer/container"
	_ "cocin_dokcer/nsenter"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

// ENV_EXEC_PID 和 ENV_EXEC_CMD 主要是为了控制是否执行C代码
const ENV_EXEC_PID = "cocin_docker_pid"
const ENV_EXEC_CMD = "cocin_docker_cmd"

// 根据提供的容器名，获取对应容器的PID 通过之前的后台运行信息来实现
func getContainerPidByName(containerName string) (string, error) {
	dirURL := fmt.Sprintf(container.DefaultInfoLocation, containerName)
	configFilePath := dirURL + container.ConfigName
	contentBytes, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return "", err
	}
	var containerInfo container.ContainerInfo
	// 将文件反序列化成容器信息对象，然后返回对应的PID
	if err := json.Unmarshal(contentBytes, &containerInfo); err != nil {
		return "", err
	}
	return containerInfo.Pid, nil
}

func ExecContainer(containerName string, comArray []string) {
	// 获取宿主机PID
	pid, err := getContainerPidByName(containerName)
	if err != nil {
		log.Errorf("Exec container getContainerPidByName %s error %v", containerName, err)
		return
	}
	// 把命令以空格为分隔符拼接成字符串，便于传递
	cmdStr := strings.Join(comArray, " ")
	log.Infof("container pid %s", pid)
	log.Infof("command %s", cmdStr)

	cmd := exec.Command("/proc/self/exe", "exec")

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	os.Setenv(ENV_EXEC_PID, pid)
	os.Setenv(ENV_EXEC_CMD, cmdStr)

	// 获得对应的PID环境变量，其实也就是容器的环境变量
	containerEnvs := getEnvsByPid(pid)
	// 宿主机的环境变量和容器的环境变量都放置到exec进程内
	cmd.Env = append(os.Environ(), containerEnvs...)

	if err := cmd.Run(); err != nil {
		log.Errorf("Exec container %s error %v", containerName, err)
	}
}

// 根据指定PID获取对应进程的环境变量
func getEnvsByPid(pid string) []string {
	// 进程环境变量存放的位置是 /proc/PID/environ
	path := fmt.Sprintf("/proc/%s/environ", pid)
	contentBytes, err := ioutil.ReadFile(path)
	if err != nil {
		log.Errorf("Read file %s error %v", path, err)
		return nil
	}
	// 多个环境变量的分隔符是 \u0000
	envs := strings.Split(string(contentBytes), "\u0000")
	return envs
}
