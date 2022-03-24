package container

import (
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"strings"
)

var (
	RUNNING             string = "running"
	STOP                string = "stopped"
	Exit                string = "exited"
	DefaultInfoLocation string = "/var/run/cocin_docker/%s/"
	ConfigName          string = "config.json"
	ContainerLogFile    string = "container.log"
)

type ContainerInfo struct {
	Pid         string `json:"pid"`        //容器的init进程在宿主机上的 PID
	Id          string `json:"id"`         //容器Id
	Name        string `json:"name"`       //容器名
	Command     string `json:"command"`    //容器内init运行命令
	CreatedTime string `json:"createTime"` //创建时间
	Status      string `json:"status"`     //容器的状态
}

/*
	NewWorkSpace 函数是用来创建容器文件系统的，包括下面三个函数
	CreateReadOnlyLayer 函数是用来新建busybox文件夹，将busybox.tar解压到busybox目录下，作为容器的只读层
	CreateWriteLayer 函数创建了一个名为writeLayer的文件夹，作为容器唯一的可写层
	CreateMountPoint 函数中，首先创建了mnt文件夹，作为挂载点，然后把writeLayer目录和busybox目录mount到mnt目录下

	最后，在NewParentProcess 函数中将容器使用的宿主机目录改成/root/mnt
*/

func NewWorkSpace(rootURL, mntURL, volume string) {
	CreateReadOnlyLayer(rootURL)
	CreateWriteLayer(rootURL)
	CreateMountPoint(rootURL, mntURL)
	// 判断volume是否为空，如果是，就表示用户没有挂载卷，结束。否则解析
	if volume != "" {
		// 解析出volume的位置和需要挂载的地方 注意目前只能挂载一个
		volumeURLs := volumeUrlExtract(volume)
		length := len(volumeURLs)
		if length == 2 && volumeURLs[0] != "" && volumeURLs[1] != "" {
			// 把volume挂载到相应的位置上
			MountVolume(rootURL, mntURL, volumeURLs)
			log.Info("%q", volumeURLs)
		} else {
			log.Infof("Volume parameter input is not correct.")
		}
	}
}

// CreateReadOnlyLayer 将busybox.tar解压到busybox目录下，作为容器的只读层
func CreateReadOnlyLayer(rootURL string) {
	busyboxURL := rootURL + "busybox/"
	busyboxTarURL := rootURL + "busybox.tar"
	exist, err := PathExists(busyboxURL)
	if err != nil {
		log.Infof("Fail to judge whether dir %s exists. %v", busyboxURL, err)
	}
	if exist == false {
		if err := os.Mkdir(busyboxURL, 0777); err != nil {
			log.Errorf("Mkdir dir %s error. %v", busyboxURL, err)
		}
		if _, err := exec.Command("tar", "-xvf", busyboxTarURL, "-C", busyboxURL).CombinedOutput(); err != nil {
			log.Errorf("unTar dir %s error %v", busyboxTarURL, err)
		}
	}
}

// CreateWriteLayer 创建名为writeLayer的文件夹作为容器唯一的可写层
func CreateWriteLayer(rootURL string) {
	writeURL := rootURL + "writeLayer/"
	if err := os.Mkdir(writeURL, 0777); err != nil {
		log.Errorf("Mkdir dir %s error. %v", writeURL, err)
	}
}

// CreateMountPoint 创建了mnt文件夹，作为挂载点，然后把writeLayer目录和busybox目录mount到mnt目录下
func CreateMountPoint(rootURL string, mntURL string) {
	// 创建mnt文件夹作为挂载点
	if err := os.Mkdir(mntURL, 0777); err != nil {
		log.Errorf("Mkdir dir %s error. %v", mntURL, err)
	}
	// 把writeLayer目录和busybox目录mount到mnt目录下
	dirs := "dirs=" + rootURL + "writeLayer:" + rootURL + "busybox"
	cmd := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", mntURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("%v", err)
	}
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
func DeleteWorkSpace(rootURL, mntURL, volume string) {
	if volume != "" {
		volumeURLs := volumeUrlExtract(volume)
		if len(volumeURLs) == 2 && volumeURLs[0] != "" && volumeURLs[1] != "" {
			DeleteMountPointWithVolume(rootURL, mntURL, volumeURLs)
		} else {
			DeleteMountPoint(rootURL, mntURL)
		}
	} else {
		DeleteMountPoint(rootURL, mntURL)
	}
	DeleteWriteLayer(rootURL)
}

func DeleteMountPoint(rootURL, mntURL string) {
	cmd := exec.Command("umount", "-A", mntURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("%v", err)
	}
	if err := os.RemoveAll(mntURL); err != nil {
		log.Errorf("Remove dir %s error %v", mntURL, err)
	}
}

func DeleteWriteLayer(rootURL string) {
	writeURL := rootURL + "writeLayer/"
	if err := os.RemoveAll(writeURL); err != nil {
		log.Errorf("Remove dir %s error %v", writeURL, err)
	}
}

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
func MountVolume(rootURL, mntURL string, volumeURLs []string) {
	// 创建宿主机文件目录 这里如果文件已经存在 不报错不退出 直接用
	parentUrl := volumeURLs[0]
	if err := os.Mkdir(parentUrl, 0777); err != nil {
		log.Infof("Mkdir parent dir %s error. %v", parentUrl, err)
	}
	// 在容器文件系统里创建挂载点
	containerUrl := volumeURLs[1]
	containerVolumeURL := mntURL + containerUrl
	if err := os.Mkdir(containerVolumeURL, 0777); err != nil {
		log.Infof("Mkdir container dir %s error. %v", containerVolumeURL, err)
	}
	// 把宿主机文件目录挂载到容器挂载点
	dirs := "dirs=" + parentUrl
	cmd := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", containerVolumeURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("Mount volume failed. %v", err)
	}
}

/*
	DeleteMountPointWithVolume 函数处理如下
	1. 首先，卸载volume挂载点的文件系统(/root/mnt/{containerUrl})，保证整个容器的挂载点没有被使用
	2. 然后，再卸载整个容器文件系统的挂载点(/root/mnt)
	3. 最后，删除容器文件系统挂载点
*/
func DeleteMountPointWithVolume(rootURL, mntURL string, volumeURLs []string) {
	containerUrl := mntURL + volumeURLs[1]
	cmd := exec.Command("umount", containerUrl)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("Umount volume failed. %v", err)
	}
	// 卸载整个容器挂载点
	cmd = exec.Command("umount", "-A", mntURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("Umount volume failed. %v", err)
	}
	// 删除容器文件系统挂载点
	if err := os.RemoveAll(mntURL); err != nil {
		log.Infof("Remove mountpoint dir %s error %v", mntURL, err)
	}
}
