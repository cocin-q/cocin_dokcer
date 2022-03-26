package network

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"net"
	"os/exec"
	"strings"
)

type BridgeNetworkDriver struct {
}

func (d *BridgeNetworkDriver) Name() string {
	return "bridge"
}

func (d *BridgeNetworkDriver) Create(subnet string, name string) (*Network, error) {
	// 取到网段字符串中的网关IP地址和网络IP范围，即带/24这种的
	ip, ipRange, _ := net.ParseCIDR(subnet)
	ipRange.IP = ip
	// 初始化网络对象
	n := &Network{
		Name:    name,
		IpRange: ipRange,
		Driver:  d.Name(),
	}
	// 配置Linux Bridge
	err := d.initBridge(n)
	if err != nil {
		log.Errorf("error init bridge: %v", err)
	}

	return n, err
}

func (d *BridgeNetworkDriver) Delete(network Network) error {
	bridgeName := network.Name
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return err
	}
	return netlink.LinkDel(br)
}

// Connect 连接一个网络和网络端点
func (d *BridgeNetworkDriver) Connect(network *Network, endpoint *Endpoint) error {
	// 获得Linux Bridge的接口名
	bridgeName := network.Name
	// 获得接口对象和属性
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return err
	}

	// 创建Veth接口的配置
	la := netlink.NewLinkAttrs()
	// Veth当前端的接口名
	la.Name = endpoint.ID[:5]
	// 通过设置Veth接口的master属性，设置这个Veth的一端挂载到网络对应的Linux Bridge上
	la.MasterIndex = br.Attrs().Index

	// 创建Veth对象，通过PeerName配置Veth另外一端的接口名
	endpoint.Device = netlink.Veth{
		LinkAttrs: la,
		PeerName:  "cif-" + endpoint.ID[:5],
	}

	// 调用netlink的LinkAdd方法创建出这个Veth接口
	// 因为上面指定了link的MasterIndex是网络对应的Linux Bridge
	// 所以Veth的一端就已经挂载到了网络对应的Linux Bridge上
	if err = netlink.LinkAdd(&endpoint.Device); err != nil {
		return fmt.Errorf("Error Add Endpoint Device: %v", err)
	}

	// 设置Veth启动 相当于ip link set xxx up命令
	if err = netlink.LinkSetUp(&endpoint.Device); err != nil {
		return fmt.Errorf("Error Add Endpoint Device: %v", err)
	}
	return nil
}

func (d *BridgeNetworkDriver) Disconnect(network Network, endpoint *Endpoint) error {
	return nil
}

func (d *BridgeNetworkDriver) initBridge(n *Network) error {
	// 创建Bridge虚拟设备
	bridgeName := n.Name
	if err := createBridgeInterface(bridgeName); err != nil {
		return fmt.Errorf("Error add bridge： %s, Error: %v", bridgeName, err)
	}

	// 设置Bridge设备的地址和路由
	gatewayIP := *n.IpRange
	gatewayIP.IP = n.IpRange.IP

	if err := setInterfaceIP(bridgeName, gatewayIP.String()); err != nil {
		return fmt.Errorf("Error assigning address: %s on bridge: %s with an error of: %v", gatewayIP, bridgeName, err)
	}

	// 启动Bridge设备
	if err := setInterfaceUP(bridgeName); err != nil {
		return fmt.Errorf("Error set bridge up: %s, Error: %v", bridgeName, err)
	}

	// 设置iptables的SNAT规则
	if err := setupIPTables(bridgeName, n.IpRange); err != nil {
		return fmt.Errorf("Error setting iptables for %s: %v", bridgeName, err)
	}

	return nil
}

// 创建Bridge虚拟设备
func createBridgeInterface(bridgeName string) error {
	// 先检查是否存在同名设备
	_, err := net.InterfaceByName(bridgeName)
	// 存在或者报错 返回
	if err == nil || !strings.Contains(err.Error(), "no such network interface") {
		return err
	}

	// 初始化一个netlink的Link基础对象，Link的名字即Bridge虚拟设备的名字
	la := netlink.NewLinkAttrs()
	la.Name = bridgeName

	// 创建netlink的Bridge对象
	br := &netlink.Bridge{LinkAttrs: la}
	// 创建Bridge虚拟网络设备，相当于 ip link add xxxx
	if err := netlink.LinkAdd(br); err != nil {
		return fmt.Errorf("Bridge creation failed for bridge %s: %v", bridgeName, err)
	}
	return nil
}

// 设置Bridge设备的地址和路由 设置一个网络接口的IP地址
func setInterfaceIP(name string, rawIP string) error {
	// 找到需要设置的网络接口
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("error get interface: %v", err)
	}

	// netlink.ParseIPNet是对net.ParseCIDR的一个封装，可以将net.ParseCIDR的返回值中的IP和net整合
	// 比如，返回值中既包含了网段的信息，192.168.0.0/24  也包含了原始的ip 192.168.0.1
	ipNet, err := netlink.ParseIPNet(rawIP)
	if err != nil {
		return err
	}

	// 给网络接口配置地址，相当于ip addr add xxxx的命令
	// 同时如果还配置了地址所在网段的信息，比如XXX.XXX.XXX.XXX/XX，还会配置路由表XXX.XXX.XXX.XXX/XX转发到这个网络接口上
	addr := &netlink.Addr{
		IPNet: ipNet,
		Label: "",
		Flags: 0,
		Scope: 0,
		Peer:  nil,
	}
	return netlink.AddrAdd(iface, addr)
}

// 启动Bridge设备
func setInterfaceUP(interfaceName string) error {
	iface, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return fmt.Errorf("Error retrieving a link named [ %s ]: %v", iface.Attrs().Name, err)
	}

	// 设置接口状态为UP，等价于ip link set XXX up
	if err := netlink.LinkSetUp(iface); err != nil {
		return fmt.Errorf("Error enabling interface for %s: %v", interfaceName, err)
	}
	return nil
}

// 设置iptables Linux Bridge SNAT规则
// 设置 MASQUERADE规则
func setupIPTables(bridgeName string, subnet *net.IPNet) error {
	// Go中没有直接操控iptables操作的库，所以需要通过命令的方式来配置
	// 创建iptables的命令如下
	// iptables -t nat -A POSTROUTING -s address[/mask][...] ! -o <bridgeName> -j MASQUERADE
	iptablesCmd := fmt.Sprintf("-t nat -A POSTROUTING -s %s ! -o %s -j MASQUERADE", subnet.String(), bridgeName)
	cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)
	// 执行命令配置规则
	output, err := cmd.Output()
	if err != nil {
		log.Errorf("iptables Output, %v", output)
	}
	return err
}

// 删除网络对应的Linux Bridge设备
func (d *BridgeNetworkDriver) deleteBridge(n *Network) error {
	bridgeName := n.Name

	// get the link
	l, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("Getting link with name %s failed: %v", bridgeName, err)
	}

	// delete the link
	if err := netlink.LinkDel(l); err != nil {
		return fmt.Errorf("Failed to remove bridge interface %s delete: %v", bridgeName, err)
	}

	return nil
}
