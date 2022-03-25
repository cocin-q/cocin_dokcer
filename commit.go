package main

import (
	"cocin_dokcer/container"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os/exec"
)

// 制作{imageName}.tar的镜像
func commitContainer(containerName, imageName string) {
	mntURL := fmt.Sprintf(container.MntUrl, containerName)
	mntURL += "/"
	imageTar := container.RootUrl + "/" + imageName + ".tar"
	// -c 是压缩， -x 是解压 -v 是输出详细过程
	if _, err := exec.Command("tar", "-czf", imageTar, "-C", mntURL, ".").CombinedOutput(); err != nil {
		log.Errorf("Tar folder %s error %v", mntURL, err)
	}
}
