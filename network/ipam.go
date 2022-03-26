package network

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"net"
	"os"
	"path"
	"strings"
)

const ipamDefaultAllocatorPath = "/var/run/cocin_docker/network/ipam/subnet.json"

// IPAM 存放IP地址的分配信息
type IPAM struct {
	SubnetAllocatorPath string             // 分配文件存放位置
	Subnets             *map[string]string // 网段和位图算法的数组map，key是网段，value是分配的位图数组
}

var ipAllocator = &IPAM{SubnetAllocatorPath: ipamDefaultAllocatorPath}

// 加载网段地址分配信息
func (ipam *IPAM) load() error {
	if _, err := os.Stat(ipam.SubnetAllocatorPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		} else {
			return err
		}
	}
	// 打开并读取存储文件
	subnetConfigFile, err := os.Open(ipam.SubnetAllocatorPath)
	defer subnetConfigFile.Close()
	if err != nil {
		return err
	}
	subnetJson := make([]byte, 2000)
	n, err := subnetConfigFile.Read(subnetJson)
	if err != nil {
		return err
	}

	err = json.Unmarshal(subnetJson[:n], ipam.Subnets)
	if err != nil {
		log.Errorf("Error dump allocation info, %v", err)
		return err
	}
	return nil
}

// 存储网段地址分配信息
func (ipam *IPAM) dump() error {
	// 判断文件夹是否存在 path.Split函数能够分隔目录和文件，返回的是path = dir+file.
	ipamConfigFileDir, _ := path.Split(ipam.SubnetAllocatorPath)
	if _, err := os.Stat(ipamConfigFileDir); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(ipamConfigFileDir, 0644)
		} else {
			return err
		}
	}
	// 打开文件，O_TRUNC 表示如果存在则清空  O_CREATE 表示如果不存在則創建
	subnetConfigFile, err := os.OpenFile(ipam.SubnetAllocatorPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	defer subnetConfigFile.Close()
	if err != nil {
		return err
	}

	ipamConfigJson, err := json.Marshal(ipam.Subnets)
	if err != nil {
		return err
	}

	_, err = subnetConfigFile.Write(ipamConfigJson)
	if err != nil {
		return err
	}

	return nil
}

// Allocate 在網段中分配一個可用的IP地址
func (ipam *IPAM) Allocate(subnet *net.IPNet) (ip net.IP, err error) {
	// 存放網段中地址分配信息的數組
	ipam.Subnets = &map[string]string{}

	// 从文件中加载已经分配的网段信息
	err = ipam.load()
	if err != nil {
		log.Errorf("Error dump allocation info, %v", err)
	}

	// 必须要加！！！
	_, subnet, _ = net.ParseCIDR(subnet.String())

	// 比如127.0.0.0/8 子網掩碼是255.0.0.0 返回8 和 32，8就是網段前面固定位的長度，32就是子網掩碼長度，應該是分辨ipv4還是ipv6
	one, size := subnet.Mask.Size()

	// 如果之前沒分配過這個網段，分配
	if _, exist := (*ipam.Subnets)[subnet.String()]; !exist {
		// 一開始都是未分配，0填充。
		// uint8(size - one) 是表示後面那些可分配位數， 2 ^ uint8(size - one)就是可用IP
		(*ipam.Subnets)[subnet.String()] = strings.Repeat("0", 1<<uint8(size-one))
	}
	// 遍历网段的位图数组
	for c := range (*ipam.Subnets)[subnet.String()] {
		// 找到为0的项和数组序号，分配IP
		if (*ipam.Subnets)[subnet.String()][c] == '0' {
			// 把它设置为1 分配
			ipalloc := []byte((*ipam.Subnets)[subnet.String()])
			ipalloc[c] = '1'
			(*ipam.Subnets)[subnet.String()] = string(ipalloc)
			// 这个IP是一个初始IP，比如192.168.0.0/16 那这里就是192.168.0.0
			ip = subnet.IP

			// IP地址是uint的一个数组，需要通过数组中的每一项加上所需要的值
			for t := uint(4); t > 0; t -= 1 {
				[]byte(ip)[4-t] += uint8(c >> ((t - 1) * 8))
			}

			// 由于IP地址从1开始分配，所以最后再加1   ???  这里似乎不对，可能会分配出256这样的地址
			ip[3]++
			break
		}
	}
	ipam.dump()
	return
}

func (ipam *IPAM) Release(subnet *net.IPNet, ipaddr *net.IP) error {
	ipam.Subnets = &map[string]string{}

	// 必须要加！！！
	_, subnet, _ = net.ParseCIDR(subnet.String())

	err := ipam.load()
	if err != nil {
		log.Errorf("Error dump allocation info, %v", err)
	}

	// 计算IP地址在网段位图数组中的索引位置
	c := 0
	releaseIP := ipaddr.To4()
	// 由于IP是从1开始分配的，所以转换成索引应该减1
	releaseIP[3] -= 1
	for t := uint(4); t > 0; t -= 1 {
		c += int(releaseIP[t-1]-subnet.IP[t-1]) << ((4 - t) * 8)
	}

	ipalloc := []byte((*ipam.Subnets)[subnet.String()])
	ipalloc[c] = '0'
	(*ipam.Subnets)[subnet.String()] = string(ipalloc)

	ipam.dump()
	return nil
}
