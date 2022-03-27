package network

import (
	"cocin_dokcer/container"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"
)

var (
	defaultNetworkPath = "/var/run/cocin_docker/network/network/"
	drivers            = map[string]NetworkDriver{}
	networks           = map[string]*Network{}
)

type Network struct {
	Name    string     // 网络名
	IpRange *net.IPNet // 地址段
	Driver  string     // 网络驱动名
}

type Endpoint struct {
	ID          string           `json:"id"`
	Device      netlink.Veth     `json:"dev"`
	IPAddress   net.IP           `json:"ip"`
	MacAddress  net.HardwareAddr `json:"mac"`
	PortMapping []string
	Network     *Network
}

type NetworkDriver interface {
	Name() string                                         // 驱动名
	Create(subnet string, name string) (*Network, error)  // 创建网络
	Delete(network Network) error                         // 删除网络
	Connect(network *Network, endpoint *Endpoint) error   // 连接容器网络端点到网络
	Disconnect(network Network, endpoint *Endpoint) error // 从网络上移除容器网络端点
}

// CreateNetwork 创建网络
func CreateNetwork(driver, subnet, name string) error {
	// ParseCIDR 的功能是将网段的字符串转换成net.IPNet 的对象
	// For example, ParseCIDR("192.0.2.1/24")
	// returns the IP address 192.0.2.1 and the network 192.0.2.0/24.
	_, cidr, _ := net.ParseCIDR(subnet)
	// 通过IPAM分配网关IP，获取到网段中第一个IP作为网关IP。和普通分配IP的流程一样的。
	gatewayIP, err := ipAllocator.Allocate(cidr)
	if err != nil {
		return err
	}
	cidr.IP = gatewayIP

	// 调用指定的网络驱动创建网络， 这里的drivers字典是各个网络驱动的实例字典
	// 通过调用网络驱动的Create方法创建网络，目前主要创建的是Bridge驱动
	nw, err := drivers[driver].Create(cidr.String(), name)
	if err != nil {
		return err
	}
	//保存网络信息，将网络的信息保存在文件系统中，以便查询和在网络上连接网络端点
	return nw.dump(defaultNetworkPath)
}

// dump 将这个网络的配置信息保存在文件系统中
func (nw *Network) dump(dumpPath string) error {
	// 检查保存的目录是否存在，不存在则创建
	if _, err := os.Stat(dumpPath); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(dumpPath, 0644)
		} else {
			return err
		}
	}

	// 保存的文件名是网络的名字
	nwPath := path.Join(dumpPath, nw.Name)
	// 打开保存的文件用于写入，后面打开的模式参数分别是 存在内容则清空、只写入、不存在则创建
	nwFile, err := os.OpenFile(nwPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		logrus.Errorf("error：", err)
		return err
	}
	defer nwFile.Close()

	// 序列化网络对象
	nwJson, err := json.Marshal(nw)
	if err != nil {
		logrus.Errorf("error：", err)
		return err
	}
	// 把序列化内容写入文件
	_, err = nwFile.Write(nwJson)
	if err != nil {
		logrus.Errorf("error：", err)
		return err
	}
	return nil
}

// load 加载网络配置
func (nw *Network) load(dumpPath string) error {
	// 打开配置文件 只读的方式
	nwConfigFile, err := os.Open(dumpPath)
	defer nwConfigFile.Close()
	if err != nil {
		return err
	}
	// 从配置文件中读取网络的配置json字符串
	nwJson := make([]byte, 2000)
	n, err := nwConfigFile.Read(nwJson)
	if err != nil {
		return err
	}
	// 反序列化
	if err = json.Unmarshal(nwJson[:n], nw); err != nil {
		logrus.Errorf("Error load nw info", err)
		return err
	}
	return nil
}

/*
	将容器的网络端点加入到容器的网络空间中
	并锁定当前程序所执行的线程，使当前线程进入到容器的网络空间
	返回值是一个函数指针，执行完这个返回函数才会退出容器的网络空间，回到宿主机的网络空间
*/
// 进入容器网络命名空间
func enterContainerNetns(enLink *netlink.Link, cinfo *container.ContainerInfo) func() {
	f, err := os.OpenFile(fmt.Sprintf("/proc/%s/ns/net", cinfo.Pid), os.O_RDONLY, 0)
	if err != nil {
		logrus.Errorf("error get container net namespace, %v", err)
	}

	// 通过这个文件描述符NSFD可以操作容器的网络空间
	nsFD := f.Fd()
	// 调用 LockOSThread 将绑定当前 goroutine 到当前操作系统线程，
	// 此 goroutine 将始终在此线程执行，
	// 其它 goroutine 则无法在此线程中得到执行，
	//直到当前调用线程执行了 UnlockOSThread 为止
	//（也就是说指定一个goroutine 独占 一个系统线程）；
	/*
		锁定当前程序所执行的线程，如果不锁定操作系统线程的话，Go语言的goroutine可能会被调度到别的线程上去
		就不能保证一直在所需要的网络空间中了
	*/
	runtime.LockOSThread()

	// 修改veth peer 另外一端移到容器的namespace中
	if err = netlink.LinkSetNsFd(*enLink, int(nsFD)); err != nil {
		logrus.Errorf("error set link netns , %v", err)
	}

	// 获取当前的网络namespace，即宿主机的网络空间
	origns, err := netns.Get()
	if err != nil {
		logrus.Errorf("error get current netns, %v", err)
	}

	// 设置当前进程到容器的网络namespace
	if err = netns.Set(netns.NsHandle(nsFD)); err != nil {
		logrus.Errorf("error set netns, %v", err)
	}
	return func() {
		// 执行完容器配置，恢复到宿主机的namespace
		netns.Set(origns)
		origns.Close()
		runtime.UnlockOSThread()
		f.Close()
	}
}

// 进入到容器的网络Namespace配置容器网络设备的IP地址和路由器
func configEndpointIpAddressAndRoute(ep *Endpoint, cinfo *container.ContainerInfo) error {
	// 获得Veth的另一端
	peerLink, err := netlink.LinkByName(ep.Device.PeerName)
	if err != nil {
		return fmt.Errorf("fail config endpoint: %v", err)
	}

	// 将容器网络端点加入网络空间 使得这些配置操作全在这个网络空间中进程，执行完函数后，恢复默认的网络空间
	defer enterContainerNetns(&peerLink, cinfo)()

	// 获取到容器的IP地址及网段，用于配置容器内部接口地址
	// 比如容器IP是192.168.1.2 网络的网段是192.168.1.0/24
	// 那么这里的IP字符串就是192.168.1.2/24，用于容器内Veth端点的配置
	interfaceIP := *ep.Network.IpRange
	interfaceIP.IP = ep.IPAddress

	// 设置容器内Veth端点的IP
	if err = setInterfaceIP(ep.Device.PeerName, interfaceIP.String()); err != nil {
		return fmt.Errorf("%v,%s", ep.Network, err)
	}

	// 启动容器内的Veth端点
	if err = setInterfaceUP(ep.Device.PeerName); err != nil {
		return err
	}

	// Net Namespace中默认本地地址127.0.0.1 的 lo 网卡是关闭状态的
	// 启动它以确保容器访问自己的请求
	if err = setInterfaceUP("lo"); err != nil {
		return err
	}

	// 设置容器内的外部请求都通过容器内的Veth端点访问。默认路由，即容器内的所有包流向默认流到这
	_, cidr, _ := net.ParseCIDR("0.0.0.0/0")

	// 构建要添加的路由数据，包括网络设备、网关IP及目的网段
	// 相当于 route add -net 0.0.0.0/0  gw （Bridge网桥地址） dev（容器内的Veth端点设备）
	defaultRoute := &netlink.Route{
		LinkIndex: peerLink.Attrs().Index,
		Gw:        ep.Network.IpRange.IP, // 网关
		Dst:       cidr,
	}

	// 添加路由到容器的网络空间 RouteAdd函数相当于route add命令
	if err = netlink.RouteAdd(defaultRoute); err != nil {
		return err
	}

	return nil
}

// 配置端口映射
func configPortMapping(ep *Endpoint, cinfo *container.ContainerInfo) error {
	for _, pm := range ep.PortMapping {
		// 宿主机端口:容器端口
		portMapping := strings.Split(pm, ":")
		if len(portMapping) != 2 {
			logrus.Errorf("port mapping format error, %v", pm)
			continue
		} else {
			logrus.Infof("映射%s --> %s", portMapping[0], portMapping[1])
		}
		// 把宿主机的端口请求转发到容器的地址和端口上
		iptablesCmd := fmt.Sprintf("-t nat -A PREROUTING -p tcp -m tcp --dport %s -j DNAT --to-destination %s:%s",
			portMapping[0], ep.IPAddress.String(), portMapping[1])
		cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)
		//err := cmd.Run()
		output, err := cmd.Output()
		if err != nil {
			logrus.Errorf("iptables Output, %v", output)
			continue
		}
		logrus.Infof("输出：%s", cmd.String())
	}
	return nil
}

// Connect 连接到容器之前创建的网络中
func Connect(networkName string, cinfo *container.ContainerInfo) error {
	// 从networks字典中取出容器连接的网络的信息，networks字典中保存了当前已经创建的网络
	network, ok := networks[networkName]
	if !ok {
		return fmt.Errorf("No Such Network: %s", networkName)
	}
	// 通过调用IPAM从网络的网段中获得可用的IP作为容器IP地址
	ip, err := ipAllocator.Allocate(network.IpRange)
	if err != nil {
		return err
	}

	// 创建网络端点
	ep := &Endpoint{
		ID:          fmt.Sprintf("%s-%s", cinfo.Id, networkName),
		IPAddress:   ip,
		PortMapping: cinfo.PortMapping,
		Network:     network,
	}

	// 调用网络驱动的Connect方法去连接和配置网络端点，这里以Bridge为例
	if err = drivers[network.Driver].Connect(network, ep); err != nil {
		return err
	}

	// 进入到容器的网络Namespace配置容器网络设备的IP地址和路由器
	if err = configEndpointIpAddressAndRoute(ep, cinfo); err != nil {
		return err
	}

	// 配置容器到宿主机的端口映射
	return configPortMapping(ep, cinfo)
}

// Init 从网络配置的目录中加载所有的网络配置信息到networks字典中
func Init() error {
	// 加载网络驱动	目前只实现Bridge方式的
	var bridgeDriver = BridgeNetworkDriver{}
	drivers[bridgeDriver.Name()] = &bridgeDriver

	// 判断网络的配置目录是否存在，不存在则创建
	if _, err := os.Stat(defaultNetworkPath); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(defaultNetworkPath, 0644)
		} else {
			return err
		}
	}

	// 检查网络配置目录中的所有文件
	// filepath.Walk(path, func(string, os.FileInfo, error)) 函数会遍历指定的path目录
	// 并执行第二个参数中的函数指针去处理目录下的每一个文件
	filepath.Walk(defaultNetworkPath, func(nwPath string, info os.FileInfo, err error) error {
		// 如果是目录，跳过
		if info.IsDir() {
			return nil
		}

		// 加载文件名作为网络名
		_, nwName := path.Split(nwPath)
		nw := &Network{Name: nwName}

		// 调用load方法加载网络配置信息
		if err := nw.load(nwPath); err != nil {
			logrus.Errorf("error load network: %s", err)
		}

		// 将网络的配置信息加入到networks字典中
		networks[nwName] = nw
		return nil
	})
	return nil
}

// ListNetwork 其实就是遍历那个networks字典
func ListNetwork() {
	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	fmt.Fprint(w, "NAME\tIpRange\tDriver\n")
	for _, nw := range networks {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			nw.Name,
			nw.IpRange.String(),
			nw.Driver,
		)
	}
	if err := w.Flush(); err != nil {
		logrus.Errorf("Flush error %v", err)
		return
	}
}

func DeleteNetwork(networkName string) error {
	// 查找网络是否存在
	nw, ok := networks[networkName]
	if !ok {
		return fmt.Errorf("No Such Network: %s", networkName)
	}
	// 释放网络网关的IP
	if err := ipAllocator.Release(nw.IpRange, &nw.IpRange.IP); err != nil {
		return fmt.Errorf("Error Remove Network gateway ip: %s", err)
	}

	// 调用网络驱动删除网络创建的设备与配置
	if err := drivers[nw.Driver].Delete(*nw); err != nil {
		return fmt.Errorf("Error Remove Network DriverError: %s", err)
	}

	// 删除该网络对应的配置文件
	return nw.remove(defaultNetworkPath)
}

func (nw *Network) remove(dumpPath string) error {
	if _, err := os.Stat(path.Join(dumpPath, nw.Name)); err != nil {
		if os.IsNotExist(err) {
			return nil
		} else {
			return err
		}
	} else {
		return os.Remove(path.Join(dumpPath, nw.Name))
	}
}
