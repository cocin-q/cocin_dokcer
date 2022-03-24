package subsystems

// ResourceConfig 用于传递资源配置的结构体
type ResourceConfig struct {
	MemoryLimit string // 内存限制
	CpuShare    string // CPU时间片权重
	CpuSet      string // CPU核心数
}

// Subsystem 接口，每个Subsystem可以实现下面的4个接口
// 这里把cgroup抽象成了path，即字符串，一个路径。
// 原因是cgroup在Hierarchy的路径，下面包含它的限制文件
type Subsystem interface {
	Name() string                               // 返回subsystem的名字，比如CPU、memory
	Set(path string, res *ResourceConfig) error // 设置某个cgroup在这个Subsystem中的资源限制
	Apply(path string, pid int) error           // 将进程添加到某个cgroup中
	Remove(path string) error                   // 移除某个cgroup
}

// SubsystemIns 通过不同的subsystem初始化实例创建资源限制处理链数组
var SubsystemIns = []Subsystem{
	&CpusetSubSystem{},
	&MemorySubSystem{},
	&CpuSubSystem{},
}
