package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"slices"
	"time"

	"gopkg.in/yaml.v3"
)

type ImportEndpoint struct {
	Endpoint string `yaml:"endpoint"`
	Port     int    `yaml:"server_port"`
}

type Server struct {
	Status       bool              `yaml:"status"`
	Debug        bool              `yaml:"debug"`
	MakeCert     bool              `yaml:"make_cert"`
	MakeCSV      bool              `yaml:"make_csv"`
	AuthModes    []string          `yaml:"auth_modes"`
	TrustedCerts []string          `yaml:"trusted_certs"`
	TrustedKeys  []string          `yaml:"trusted_keys"`
	Endpoints    []ImportEndpoint  `yaml:"endpoints"`
	Security     map[string]string `yaml:"security"`
}

type Device struct {
	Name         string   `json:"name" yaml:"name"`
	Address      string   `json:"address" yaml:"address"`
	Port         int      `json:"port" yaml:"port"`
	DelayMs      int      `json:"delay_ms" yaml:"delay_ms"`
	TagsPackName string   `yaml:"tags_pack_name"`
	TagsPack     []string `json:"tags_pack" yaml:"tags_pack"`
}

type Config struct {
	Logfile       bool                         `json:"logfile" yaml:"logfile"`
	HandleTimeout int                          `json:"handle_timeout" yaml:"handle_timeout"`
	Server        Server                       `json:"server" yaml:"server"`
	Devices       []Device                     `json:"devices" yaml:"devices"`
	TagPacks      map[string]map[string]string `yaml:"tag_packs"`
}

var config Config
var plugin_dir string

var log_buf bytes.Buffer
var logger *log.Logger
var log_file *os.File

func InitCrashLog() {
	logger.Println("Завершение плагина")
	if r := recover(); r != nil && log_buf.Len() > 0 {
		logger.Println("Panic:", r)
		trace := debug.Stack()
		logger.Println(string(trace))
		plugin_path, err := os.Executable()
		if err == nil {
			dir := filepath.Dir(plugin_path)
			crash_path := filepath.Join(dir, "crash.log")
			if err := os.WriteFile(crash_path, log_buf.Bytes(), 0644); err != nil {
				logger.Println("Ошибка записи файла crash.log:", err)
			}
		}
		os.Exit(1)
	}
}

func InitLogFile() {
	logger.Println("Завершение плагина")
	if r := recover(); r != nil && log_buf.Len() == 0 {
		logger.Println("Panic:", r)
		trace := debug.Stack()
		logger.Println(string(trace))
		log_file.Close()
		os.Exit(1)
	}
}

func main() {
	multi_writer := io.MultiWriter(os.Stdout, &log_buf)
	logger = log.New(multi_writer, "Plugin: ", log.Ldate|log.Ltime|log.Lshortfile)
	logger.Println("Запуск плагина")
	defer InitCrashLog()

	plugin_path, err := os.Executable()
	if err != nil {
		logger.Panicf("Ошибка при определении пути исполняемого файла %v", err)
	}
	plugin_dir = filepath.Dir(plugin_path)

	data_path := filepath.Join(plugin_dir, "plugin.conf")
	file_content, err := os.ReadFile(data_path)
	if err != nil {
		logger.Panicf("Ошибка чтения файла %v", err)
	}
	err = yaml.Unmarshal(file_content, &config)
	if err != nil {
		logger.Panicf("Ошибка чтения plugin.conf (yaml) %v", err)
	}

	if config.Logfile {
		log_path := filepath.Join(plugin_dir, "plugin.log")
		log_file, err = os.OpenFile(log_path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			logger.Panicf("Ошибка открытия файла логов %v", err)
		}
		multi_writer := io.MultiWriter(os.Stdout, log_file)
		logger = log.New(multi_writer, "Plugin: ", log.Ldate|log.Ltime|log.Lshortfile)
		if log_buf.Len() > 0 {
			logger.Println(log_buf.String())
		}
		log_buf.Reset()
		defer InitLogFile()
	}

	if len(config.Devices) == 0 {
		logger.Panicln("Добавьте устройства для сбора данных")
	}

	var device_names []string
	var device_addresses []string
	for _, device := range config.Devices {
		if slices.Contains(device_names, device.Name) {
			logger.Panicf("Устройство с именем %s уже существует", device.Name)
		}
		if slices.Contains(device_addresses, device.Address) {
			logger.Panicf("Устройство с адресом %s уже существует", device.Name)
		}
		device_names = append(device_names, device.Name)
		device_addresses = append(device_addresses, device.Address)
	}

	if config.Server.Status {
		LoadTagPacks()
		go StartServer()
	}

	json_devices, err := json.Marshal(config.Devices)
	if err != nil {
		logger.Panicf("Ошибка формирования списка устройств (Json) %v", err)
	}

	collector_path := filepath.Join(plugin_dir, "collector.exe")
	pid := os.Getpid()
	cmd := exec.Command(
		collector_path, string(json_devices),
		fmt.Sprintf("%d", pid),
		fmt.Sprintf("%d", config.HandleTimeout))
	AddProcessToGroup(cmd)
	stderr_pipe, err := cmd.StderrPipe()
	if err != nil {
		logger.Panicf("Ошибка получения StderrPipe %v", err)
	}
	stdout_pipe, err := cmd.StdoutPipe()
	if err != nil {
		logger.Panicf("Ошибка получения StdoutPipe %v", err)
	}
	if err := cmd.Start(); err != nil {
		logger.Panicf("Ошибка запуска сборщика %v", err)
	}
	logger.Println("collector.exe запущен")
	go func() {
		stderr_scanner := bufio.NewScanner(stderr_pipe)
		for stderr_scanner.Scan() {
			logger.Println("collector error: ", stderr_scanner.Text())
		}
	}()
	scanner := bufio.NewScanner(stdout_pipe)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		if json.Valid([]byte(line)) {
			if config.Server.Status {
				UpdateCollector(line)
			}
			fmt.Fprintln(os.Stdout, line)
		}
	}
	if err := scanner.Err(); err != nil {
		logger.Println("Ошибка чтения данных сборщика: ", err)
	}

	device_count := len(config.Devices)
	ShutdownCollector(cmd, time.Second*time.Duration(device_count*10))
}
