package Cgroups

import (
	"cocin_dokcer/Cgroups/subsystems"
	"github.com/sirupsen/logrus"
)

type CgroupManager struct { // 这里给出一个相对路径就行，因为subsystem的位置是固定的
	Path     string                     // cgroup在hierarchy中的路径 相当于创建的cgroup目录相对于root cgroup目录的路径
	Resource *subsystems.ResourceConfig // 资源配置
}

func NewCgroupManager(path string) *CgroupManager {
	return &CgroupManager{Path: path}
}

// Apply 将进程PID加入到每个cgroup中
func (c *CgroupManager) Apply(pid int) error {
	for _, subSysIns := range subsystems.SubsystemIns {
		subSysIns.Apply(c.Path, pid)
	}
	return nil
}

// Set 设置各个subsystem挂载中的cgroup资源限制
func (c *CgroupManager) Set(res *subsystems.ResourceConfig) error {
	for _, subSysIns := range subsystems.SubsystemIns {
		subSysIns.Set(c.Path, res)
	}
	return nil
}

// Destroy 释放各个subsystem挂载中的cgroup
func (c *CgroupManager) Destroy() error {
	for _, subSysIns := range subsystems.SubsystemIns {
		if err := subSysIns.Remove(c.Path); err != nil {
			logrus.Warnf("remove cgroup fail %v", err)
		}
	}
	return nil
}
