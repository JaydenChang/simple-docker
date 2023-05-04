package dockerCommand

import (
	"os"
	"simple-docker/cgroup"
	"simple-docker/cgroup/subsystem"
	"simple-docker/container"
	"strings"

	"github.com/sirupsen/logrus"
)

// dockerCommand/run.go
// This is the function what `docker run` will call
func Run(tty bool, containerCmd []string, res *subsystem.ResourceConfig) {

	// this is "docker init <containerCmd>"
	initProcess, writePipe := container.NewParentProcess(tty)
	if initProcess == nil {
		logrus.Errorf("new parent process error")
		return
	}

	// start the init process
	if err := initProcess.Start(); err != nil {
		logrus.Error(err)
	}

	// create container manager to control resource config on all hierarchies
	cm := cgroup.NewCgroupManager("simple-docker-container")
	defer cm.Remove()
	cm.Set(res)
	cm.AddProcess(initProcess.Process.Pid)

	// send command to write side
	// will close the plug
	sendInitCommand(containerCmd, writePipe)

	initProcess.Wait()
	rootURL := "/root/"
	mntURL := "/root/mnt/"
	container.DeleteWorkSpace(rootURL, mntURL)	
	os.Exit(0)
}

func sendInitCommand(containerCmd []string, writePipe *os.File) {
	cmdString := strings.Join(containerCmd, " ")
	logrus.Infof("whole init command is: %v", cmdString)
	writePipe.WriteString(cmdString)
	writePipe.Close()
}
