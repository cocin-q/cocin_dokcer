package container

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
)

var (
	RUNNING             string = "running"
	STOP                string = "stopped"
	Exit                string = "exited"
	DefaultInfoLocation string = "/var/run/cocin_docker/%s/"
	ConfigName          string = "config.json"
	ContainerLogFile    string = "container.log"

	RootUrl       string = "/root"
	MntUrl        string = "/root/mnt/%s"
	WriteLayerUrl string = "/root/writeLayer/%s"
)

type ContainerInfo struct {
	Pid         string `json:"pid"`        //容器的init进程在宿主机上的 PID
	Id          string `json:"id"`         //容器Id
	Name        string `json:"name"`       //容器名
	Command     string `json:"command"`    //容器内init运行命令
	CreatedTime string `json:"createTime"` //创建时间
	Status      string `json:"status"`     //容器的状态
	Volume      string `json:"volume"`
}

/*
	NewWorkSpace 函数是用来创建容器文件系统的，包括下面三个函数
	CreateReadOnlyLayer 函数是用来新建busybox文件夹，将busybox.tar解压到busybox目录下，作为容器的只读层
	CreateWriteLayer 函数创建了一个名为writeLayer的文件夹，作为容器唯一的可写层
	CreateMountPoint 函数中，首先创建了mnt文件夹，作为挂载点，然后把writeLayer目录和busybox目录mount到mnt目录下

	最后，在NewParentProcess 函数中将容器使用的宿主机目录改成/root/mnt

	更新，为每个容器创建文件系统
*/

func NewWorkSpace(volume, imageName, containerName string) {
	CreateReadOnlyLayer(imageName)
	CreateWriteLayer(containerName)
	CreateMountPoint(containerName, imageName)
	// 判断volume是否为空，如果是，就表示用户没有挂载卷，结束。否则解析
	if volume != "" {
		// 解析出volume的位置和需要挂载的地方 注意目前只能挂载一个
		volumeURLs := volumeUrlExtract(volume)
		length := len(volumeURLs)
		if length == 2 && volumeURLs[0] != "" && volumeURLs[1] != "" {
			// 把volume挂载到相应的位置上
			MountVolume(volumeURLs, containerName)
			log.Info("%q", volumeURLs)
		} else {
			log.Infof("Volume parameter input is not correct.")
		}
	}
}

// CreateReadOnlyLayer 将busybox.tar解压到busybox目录下，作为容器的只读层
func CreateReadOnlyLayer(imageName string) error {
	unTarFolderUrl := RootUrl + "/" + imageName + "/"
	imageUrl := RootUrl + "/" + imageName + ".tar"
	exist, err := PathExists(unTarFolderUrl)
	if err != nil {
		log.Infof("Fail to judge whether dir %s exists. %v", unTarFolderUrl, err)
		return err
	}
	if exist == false {
		if err := os.Mkdir(unTarFolderUrl, 0622); err != nil {
			log.Errorf("Mkdir dir %s error. %v", unTarFolderUrl, err)
			return err
		}
		if _, err := exec.Command("tar", "-xvf", imageUrl, "-C", unTarFolderUrl).CombinedOutput(); err != nil {
			log.Errorf("unTar dir %s error %v", unTarFolderUrl, err)
			return err
		}
	}
	return nil
}

// CreateWriteLayer 创建名为writeLayer的文件夹作为容器唯一的可写层
func CreateWriteLayer(containerName string) {
	writeURL := fmt.Sprintf(WriteLayerUrl, containerName)
	if err := os.Mkdir(writeURL, 0777); err != nil {
		log.Errorf("Mkdir dir %s error. %v", writeURL, err)
	}
}

// CreateMountPoint 创建了mnt文件夹，作为挂载点，然后把writeLayer目录和busybox目录mount到mnt目录下
func CreateMountPoint(containerName, imageName string) error {
	// 创建mnt文件夹作为挂载点
	mntUrl := fmt.Sprintf(MntUrl, containerName)
	if err := os.MkdirAll(mntUrl, 0777); err != nil {
		log.Errorf("Mkdir dir %s error. %v", mntUrl, err)
		return err
	}
	// 把writeLayer目录和busybox目录mount到mnt目录下
	tmpWriteLayer := fmt.Sprintf(WriteLayerUrl, containerName)
	tmpImageLocation := RootUrl + "/" + imageName
	dirs := "dirs=" + tmpWriteLayer + ":" + tmpImageLocation
	_, err := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", mntUrl).CombinedOutput()
	if err != nil {
		log.Errorf("Run command for createing mount point failed %v", err)
		return err
	}
	return nil
}

// PathExists 判断文件路径是否存在
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

/*
	Docker会在删除容器的时候，把容器对应的Write Layer 和 Container-init Layer删除，而保留镜像所有内容。
	在这，我们先在容器退出的时候删除Write Layer。
	DeleteWorkSpace函数
	首先，在DeleteMountPoint函数中umount mnt目录
	然后 删除mnt目录
	最后，在DeleteWriteLayer函数中删除writeLayer文件夹。

	更新：
	1. 只有在volume不为空，并且使用volumeURLExtract函数解析volume字符串返回的字符数组长度为2，
	   数据均不为空的时候。执行DeleteMountPointWithVolume函数来处理。
	2. 其余情况下仍然使用前面的DeleteMountPoint函数。
*/
func DeleteWorkSpace(volume, containerName string) {
	if volume != "" {
		volumeURLs := volumeUrlExtract(volume)
		if len(volumeURLs) == 2 && volumeURLs[0] != "" && volumeURLs[1] != "" {
			DeleteMountPointWithVolume(volumeURLs, containerName)
		} else {
			DeleteMountPoint(containerName)
		}
	} else {
		DeleteMountPoint(containerName)
	}
	DeleteWriteLayer(containerName)
}

func DeleteMountPoint(containerName string) error {
	mntURL := fmt.Sprintf(MntUrl, containerName)
	_, err := exec.Command("umount", "-A", mntURL).CombinedOutput()
	if err != nil {
		log.Errorf("Unmount %s error %v", mntURL, err)
		return err
	}
	if err := os.RemoveAll(mntURL); err != nil {
		log.Errorf("Remove mountpoint dir %s error %v", mntURL, err)
		return err
	}
	return nil
}

func DeleteWriteLayer(containerName string) {
	writeURL := fmt.Sprintf(WriteLayerUrl, containerName)
	if err := os.RemoveAll(writeURL); err != nil {
		log.Errorf("Remove writeLayer dir %s error %v", writeURL, err)
	}
}
