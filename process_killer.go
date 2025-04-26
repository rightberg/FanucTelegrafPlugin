package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

func createFileInCurrentDir(filename string) error {
	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("не удалось получить путь к исполняемому файлу: %w", err)
	}
	dir := filepath.Dir(executablePath)
	filePath := filepath.Join(dir, filename)
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("не удалось создать файл: %w", err)
	}
	defer file.Close()
	fmt.Printf("Файл успешно создан: %s\n", filePath)
	return nil
}

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

func ShutdownChildProcess(cmd *exec.Cmd, cancel context.CancelFunc) {
	log.Println("Попытка корректно завершить дочерний процесс")
	if cmd.Process == nil {
		log.Println("Процесс не запущен")
		cancel()
		return
	}

	pid := uint32(cmd.Process.Pid)
	err := SendCtrlBreak(pid)
	if err != nil {
		log.Println("Ошибка отправки CTRL_BREAK_EVENT:", err)

	} else {
		log.Println("CTRL_BREAK_EVENT отправлен")
	}

	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Println("Дочерний процесс завершился с ошибкой:", err)
			createFileInCurrentDir("A GOOD ")
		} else {
			log.Println("Дочерний процесс успешно завершился")
			createFileInCurrentDir("A BD ")
		}
	case <-time.After(5 * time.Second):
		log.Println("Таймаут ожидания завершения дочернего процесса — принудительное убийство")
		if kill_err := cmd.Process.Kill(); kill_err != nil {
			log.Println("Ошибка принудительного убийства процесса:", kill_err)
		}
		<-done
	}

	cancel()
}

func AddProcessToGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
