package main

import (
	"cocin_dokcer/container"
	log "github.com/sirupsen/logrus"
	"os"
)

// Run 运行命令
func Run(tty bool, command string) {
	parent := container.NewParentProcess(tty, command)
	if err := parent.Start(); err != nil {
		log.Error(err)
	}
	parent.Wait()
	os.Exit(-1)
}
