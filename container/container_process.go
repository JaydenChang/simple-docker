package container

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	RUNNING             string = "running"
	STOP                string = "stopped"
	Exit                string = "exited"
	DefaultInfoLocation string = "/var/run/simple-docker/%s/"
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

func volumeUrlExtract(volume string) []string {
	// divide volume by ":"
	return strings.Split(volume, ":")
}

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

func NewWorkSpace(rootURL, mntURL, volume string) {
	CreateReadOnlyLayer(rootURL)
	CreateWriteLayer(rootURL)
	CreateMountPoint(rootURL, mntURL)
	if volume != "" {
		volumeURLs := volumeUrlExtract(volume)
		length := len(volumeURLs)
		if length == 2 && volumeURLs[0] != "" && volumeURLs[1] != "" {
			MountVolume(rootURL, mntURL, volumeURLs)
			logrus.Infof("%q", volumeURLs)
		} else {
			logrus.Infof("volume parameter input is not correct")
		}
	}
}

func CreateReadOnlyLayer(rootURL string) {
	busyboxURL := rootURL + "/busybox/"
	busyboxTarURL := rootURL + "/busybox.tar"
	exist, err := PathExists(busyboxURL)

	if err != nil {
		logrus.Infof("fail to judge whether dir %s exists. %v", busyboxURL, err)
	}
	if !exist {
		if err := os.Mkdir(busyboxURL, 0777); err != nil {
			logrus.Errorf("mkdir dir %s error. %v", busyboxURL, err)
		}
		if _, err := exec.Command("tar", "-xvf", busyboxTarURL, "-C", busyboxURL).CombinedOutput(); err != nil {
			logrus.Errorf("unTar dir %s error %v", busyboxTarURL, err)
		}
	}
}

func CreateWriteLayer(rootURL string) {
	writeUrl := rootURL + "/writeLayer/"
	if err := os.Mkdir(writeUrl, 0777); err != nil {
		logrus.Errorf("Mkdir dir %s error. %v", writeUrl, err)
	}
}

func CreateMountPoint(rootURL string, mntURL string) {
	// create mnt folder as mount point
	if err := os.Mkdir(mntURL, 0777); err != nil {
		logrus.Errorf("mkdir dir %s error %v", mntURL, err)
	}
	// mount 'writeLayer' and 'busybox' to 'mnt'
	dirs := "dirs=" + rootURL + "writeLayer:" + rootURL + "busybox"
	cmd := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", mntURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("%v", err)
	}
}

func MountVolume(rootURL, mntURL string, volumeURLs []string) {
	// create host file catalog
	parentURL := volumeURLs[0]
	if err := os.Mkdir(parentURL, 0777); err != nil {
		logrus.Infof("mkdir parent dir %s error. %v", parentURL, err)
	}
	// create mount point in container file system
	containerURL := volumeURLs[1]
	containerVolumeURL := mntURL + containerURL
	if err := os.Mkdir(containerVolumeURL, 0777); err != nil {
		logrus.Infof("mkdir container dir %s error. %v", containerVolumeURL, err)
	}
	// mount host file catalog to mount point in container
	dirs := "dirs=" + parentURL
	cmd := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", containerVolumeURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("mount volume failed. %v", err)
	}
}

func DeleteMountPointWithVolume(rootURL, mntURL string, volumeURLs []string) {
	// umount volume point in container
	containerURL := mntURL + volumeURLs[1]
	cmd := exec.Command("umount", containerURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("umount volume failed. %v", err)
	}
	// umount the whole point of the container
	cmd = exec.Command("umount", mntURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("umount mountpoint failed. %v", err)
	}
	if err := os.RemoveAll(mntURL); err != nil {
		logrus.Infof("remove mountpoint dir %s error %v", mntURL, err)
	}
}

func DeleteMountPoint(rootURL string, mntURL string) {
	cmd := exec.Command("umount", mntURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("%v", err)
	}
	if err := os.RemoveAll(mntURL); err != nil {
		logrus.Errorf("remove dir %s error %v", mntURL, err)
	}
}

func DeleteWriteLayer(rootURL string) {
	writeURL := rootURL + "/writeLayer/"
	if err := os.RemoveAll(writeURL); err != nil {
		logrus.Errorf("remove dir %s error %v", writeURL, err)
	}
}

func DeleteWorkSpace(rootURL, mntURL, volume string) {
	if volume != "" {
		volumeURLs := volumeUrlExtract(volume)
		length := len(volumeURLs)
		if length == 2 && volumeURLs[0] != "" && volumeURLs[1] != "" {
			DeleteMountPointWithVolume(rootURL, mntURL, volumeURLs)
		} else {
			DeleteMountPoint(rootURL, mntURL)
		}
	} else {
		DeleteMountPoint(rootURL, mntURL)
	}
	DeleteWriteLayer(rootURL)
}

func randStringBytes(n int) string {
	letterBytes := "1234567890"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func NewParentProcess(tty bool, volume string, containerName string) (*exec.Cmd, *os.File) {
	readPipe, writePipe, err := os.Pipe()

	if err != nil {
		logrus.Errorf("New Pipe Error: %v", err)
		return nil, nil
	}
	// create a new command which run itself
	// the first arguments is `init` which is in the "container/init.go" file
	// so, the <cmd> will be interpret as "docker init <containerCmd>"
	cmd := exec.Command("/proc/self/exe", "init")

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS |
			syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
	}
	if tty {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		dirURL := fmt.Sprintf(DefaultInfoLocation, containerName)
		if err := os.MkdirAll(dirURL, 0622); err != nil {
			logrus.Errorf("NewParentProcess mkdir %s error %v", dirURL, err)
			return nil, nil
		}
		stdLogFilePath := dirURL + ContainerLogFile
		stdLogFile, err := os.Create(stdLogFilePath)
		if err != nil {
			logrus.Errorf("NewParentProcess create file %s error %v", stdLogFilePath, err)
			return nil, nil
		}
		cmd.Stdout = stdLogFile
	}
	cmd.ExtraFiles = []*os.File{readPipe}
	mntURL := "/root/mnt"
	rootURL := "/root/"
	NewWorkSpace(rootURL, mntURL, volume)
	cmd.Dir = mntURL

	// cmd.Dir = "/root/busybox"
	return cmd, writePipe
}