package container

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/sirupsen/logrus"
)

// container/init.go
func NewProcess(tty bool, containerCmd string) *exec.Cmd {

	// create a new command which run itself
	// the first arguments is `init` which is the below exported function
	// so, the <cmd> will be interpret as "docker init <containerCmd>"
	args := []string{"init", containerCmd}
	cmd := exec.Command("/proc/self/exe", args...)

	// new namespaces, thanks to Linux
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
	}

	// this is what presudo terminal means
	// link the container's stdio to os
	if tty {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return cmd
}

// already in container
// initiate the container
func InitProcess() error {
	// read command from pipe, will plug if write side is not ready
	containerCmd := readCommand()
	if len(containerCmd) == 0 {
		return errors.New("init process failed, containerCmd is nil")
	}

	if err := setUpMount(); err != nil {
		logrus.Errorf("initProcess look path failed: %v", err)
	}

	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV

	// mount proc filesystem
	syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")

	// look for the path of container command
	// so we don't need to type "/bin/ls", but "ls"
	path, err := exec.LookPath(containerCmd[0])
	if err != nil {
		logrus.Errorf("initProcess look path failed: %v", err)
		return err
	}

	// log path info
	// if you type "ls", it will be "/bin/ls"
	logrus.Infof("Find path: %v", path)
	if err := syscall.Exec(path, containerCmd, os.Environ()); err != nil {
		logrus.Errorf(err.Error())
	}

	return nil
}

func pivotRoot(root string) error {
	// remount the root dir, in order to make current root and old root in different file systems
	if err := syscall.Mount(root, root, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("mount rootfs to itself error: %v", err)
	}

	// create 'rootfs/.pivot_root' to store old_root
	pivotDir := filepath.Join(root, ".pivot_root")
	if err := os.Mkdir(pivotDir, 0777); err != nil {
		return err
	}

	// pivot_root mount on new rootfs, old_root mount on rootfs/.pivot_root
	if err := syscall.PivotRoot(root, pivotDir); err != nil {
		return fmt.Errorf("pivot_root %v", err)
	}

	// change current work dir to root dir
	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / %v", err)
	}

	pivotDir = filepath.Join("/", ".pivot_root")
	// umount rootfs/.rootfs_root
	if err := syscall.Unmount(pivotDir, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("umount pivot_root dir %v", err)
	}

	// del the temporary dir
	return os.Remove(pivotDir)
}

func setUpMount() error {
	// get current path
	pwd, err := os.Getwd()
	if err != nil {
		logrus.Errorf("get current location error: %v", err)
		return err
	}
	logrus.Infof("current location: %v", pwd)
	pivotRoot(pwd)

	// mount proc
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	if err := syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), ""); err != nil {
		logrus.Errorf("mount /proc failed: %v", err)
		return err
	}

	if err := syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755"); err != nil {
		logrus.Errorf("mount /dev failed: %v", err)
		return err
	}
	return nil
}
