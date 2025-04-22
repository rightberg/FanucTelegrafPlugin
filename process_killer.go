package main

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var job_handle windows.Handle

func AssignWinJobObject(cmd *exec.Cmd) (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}

	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	infoSize := uint32(unsafe.Sizeof(info))

	_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), infoSize)
	if err != nil {
		windows.CloseHandle(job)
		return 0, err
	}

	procHandle, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, uint32(cmd.Process.Pid))
	if err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	defer windows.CloseHandle(procHandle)

	err = windows.AssignProcessToJobObject(job, procHandle)
	if err != nil {
		windows.CloseHandle(job)
		return 0, err
	}

	return job, nil
}

func AddWinJobObject(cmd *exec.Cmd) {
	job, err := AssignWinJobObject(cmd)
	if err != nil {
		logger.Println("Не удалось добавить процесс в Job Object:", err)
		return
	}
	job_handle = job
}

func ShutdownChildProcess(cmd *exec.Cmd, cancel context.CancelFunc, s os.Signal) {
	logger.Printf("Получен сигнал: %s, завершаемся", s)
	cancel()
	if cmd.Process != nil {
		if err := cmd.Process.Kill(); err != nil {
			logger.Println("Не удалось завершить дочерний процесс:", err)
		}
	}
	cmd.Wait()
	if job_handle != 0 {
		windows.CloseHandle(job_handle)
	}
	os.Exit(0)
}

func AddProcessToGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
