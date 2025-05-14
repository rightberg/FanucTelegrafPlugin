package main

import (
	"os/exec"
	"syscall"
	"time"
)

func SendCtrlBreak(groupID uint32) error {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GenerateConsoleCtrlEvent")
	ret, _, err := proc.Call(
		uintptr(syscall.CTRL_BREAK_EVENT),
		uintptr(groupID),
	)
	if ret == 0 {
		return err
	}
	return nil
}

func ShutdownCollector(cmd *exec.Cmd, timeout time.Duration) {
	logger.Println("Завершаем collector.exe")
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		logger.Println("collector.exe уже завершён")
		return
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			logger.Println("collector.exe завершился с ошибкой:", err)
		} else {
			logger.Println("collector.exe завершился корректно")
		}
	case <-time.After(timeout):
		logger.Println("Время ожидания завершения истекло, принудительное завершение collector.exe")
		if err := cmd.Process.Kill(); err != nil {
			logger.Println("Не удалось принудительно завершить collector.exe:", err)
		} else {
			logger.Println("collector.exe успешно принудительно завершён")
		}
	}
}

func AddProcessToGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
