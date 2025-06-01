package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"slices"
	"sync"
	"syscall"

	"gopkg.in/yaml.v3"
)

type ImportEndpoint struct {
	Endpoint string `yaml:"endpoint"`
	Port     int    `yaml:"server_port"`
}

type Server struct {
	Status       bool                         `yaml:"status"`
	Debug        bool                         `yaml:"debug"`
	MakeCert     bool                         `yaml:"make_cert"`
	MakeCSV      bool                         `yaml:"make_csv"`
	AuthModes    []string                     `yaml:"auth_modes"`
	TrustedCerts []string                     `yaml:"trusted_certs"`
	TrustedKeys  []string                     `yaml:"trusted_keys"`
	Endpoints    []ImportEndpoint             `yaml:"endpoints"`
	Security     map[string]string            `yaml:"security"`
	TagPacks     map[string]map[string]string `yaml:"tag_packs"`
}

type Device struct {
	Name         string   `json:"name" yaml:"name"`
	Address      string   `json:"address" yaml:"address"`
	Port         int      `json:"port" yaml:"port"`
	DelayMs      int      `json:"delay_ms" yaml:"delay_ms"`
	TagsPack     []string `json:"tags_pack" yaml:"tags_pack"`
	TagsPackName string   `json:"tags_pack_name" yaml:"tags_pack_name"`
}

type Config struct {
	Logfile       bool     `json:"logfile" yaml:"logfile"`
	HandleTimeout int      `json:"handle_timeout" yaml:"handle_timeout"`
	Devices       []Device `json:"devices" yaml:"devices"`
	Server        Server   `json:"server" yaml:"server"`
}

var config Config
var plugin_dir string

var log_buf bytes.Buffer
var logger *log.Logger
var log_file *os.File

func InitCrashLog() {
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

	var wait_group sync.WaitGroup
	for index, device := range config.Devices {
		handles = append(handles, 0)
		wait_group.Add(1)
		go FanucDataCollector(device, config.HandleTimeout, &running, &handles[index], &wait_group)
	}

	go func() {
		_, err := os.Stdin.Read(make([]byte, 1))
		if err != nil {
			go EndPlugin()
		}
	}()

	end_signal := make(chan os.Signal, 1)
	signal.Notify(end_signal, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-end_signal
		go EndPlugin()
	}()

	wait_group.Wait()
	logger.Println("Окончание работы плагина: успешно")
}
