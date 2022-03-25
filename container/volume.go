package container

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"strings"
)

// 解析volume字符串
func volumeUrlExtract(volume string) []string {
	var volumeURLs []string
	volumeURLs = strings.Split(volume, ":")
	return volumeURLs
}

/*
	首先，读取宿主机文件目录URL，创建宿主机文件目录(/root/${parentUrl})
	然后，读取容器挂载点URL，在容器文件系统里创建挂载点(/root/mnt/${containerUrl})
	最后，把宿主机文件目录挂载到容器挂载点
*/
func MountVolume(volumeURLs []string, containerName string) error {
	// 创建宿主机文件目录 这里如果文件已经存在 不报错不退出 直接用
	parentUrl := volumeURLs[0]
	if err := os.Mkdir(parentUrl, 0777); err != nil {
		log.Infof("Mkdir parent dir %s error. %v", parentUrl, err)
	}
	// 在容器文件系统里创建挂载点
	containerUrl := volumeURLs[1]
	mntURL := fmt.Sprintf(MntUrl, containerName)
	containerVolumeURL := mntURL + "/" + containerUrl
	if err := os.Mkdir(containerVolumeURL, 0777); err != nil {
		log.Infof("Mkdir container dir %s error. %v", containerVolumeURL, err)
	}
	// 把宿主机文件目录挂载到容器挂载点
	dirs := "dirs=" + parentUrl
	// none就是不关联任何设备
	_, err := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", containerVolumeURL).CombinedOutput()
	if err != nil {
		log.Errorf("Mount volume failed. %v", err)
		return err
	}
	return nil
}

/*
	DeleteMountPointWithVolume 函数处理如下
	1. 首先，卸载volume挂载点的文件系统(/root/mnt/{containerUrl})，保证整个容器的挂载点没有被使用
	2. 然后，再卸载整个容器文件系统的挂载点(/root/mnt)
	3. 最后，删除容器文件系统挂载点
*/
func DeleteMountPointWithVolume(volumeURLs []string, containerName string) error {
	mntURL := fmt.Sprintf(MntUrl, containerName)
	containerUrl := mntURL + "/" + volumeURLs[1]
	if _, err := exec.Command("umount", containerUrl).CombinedOutput(); err != nil {
		log.Errorf("Umount volume %s failed. %v", containerUrl, err)
		return err
	}
	// 卸载整个容器挂载点
	if _, err := exec.Command("umount", "-A", mntURL).CombinedOutput(); err != nil {
		log.Errorf("Umount mountpoint %s failed. %v", mntURL, err)
		return err
	}
	// 删除容器文件系统挂载点
	if err := os.RemoveAll(mntURL); err != nil {
		log.Infof("Remove mountpoint dir %s error %v", mntURL, err)
		return err
	}
	return nil
}
