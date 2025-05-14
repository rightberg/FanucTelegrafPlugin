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
	"strconv"
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

func InitCrashLog() {
	defer func() {
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
	}()
}

func InitLogFile() {
	log_path := filepath.Join(plugin_dir, "plugin.log")
	log_file, err := os.OpenFile(log_path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		logger.Println("Ошибка открытия файла логов:", err)
		panic(err)
	}
	defer func() {
		logger.Println("Завершение плагина")
		if r := recover(); r != nil && log_buf.Len() == 0 {
			logger.Println("Panic:", r)
			trace := debug.Stack()
			logger.Println(string(trace))
			log_file.Close()
			os.Exit(1)
		}
		log_file.Close()
	}()
	multi_writer := io.MultiWriter(os.Stdout, log_file)
	logger = log.New(multi_writer, "Plugin: ", log.Ldate|log.Ltime|log.Lshortfile)
	if log_buf.Len() > 0 {
		logger.Println(log_buf.String())
	}
	log_buf.Reset()
}

func main() {
	multi_writer := io.MultiWriter(os.Stdout, &log_buf)
	logger = log.New(multi_writer, "Plugin: ", log.Ldate|log.Ltime|log.Lshortfile)
	logger.Println("Запуск плагина")
	InitCrashLog()

	plugin_path, err := os.Executable()
	if err != nil {
		logger.Println("Ошибка при определении пути исполняемого файла:", err)
		panic(err)
	}
	plugin_dir = filepath.Dir(plugin_path)

	data_path := filepath.Join(plugin_dir, "plugin.conf")
	file_content, err := os.ReadFile(data_path)
	if err != nil {
		logger.Println("Ошибка чтения файла:", err)
		panic(err)
	}
	err = yaml.Unmarshal(file_content, &config)
	if err != nil {
		logger.Println("Ошибка чтения plugin.conf (yaml):", err)
		panic(err)
	}

	if config.Logfile {
		InitLogFile()
	}

	for index := range config.Devices {
		str_index := strconv.Itoa(index)
		if config.Devices[index].Name == "" {
			config.Devices[index].Name = "Device " + str_index
		}
	}

	if config.Server.Status {
		InitServer()
		go StartServer()
	}

	for index := range config.Devices {
		for pack_name, tags_map := range config.TagPacks {
			var tags_pack []string
			if pack_name == config.Devices[index].TagsPackName {
				for tag := range tags_map {
					proc_tag := GetStrSliceByDot(tag)[0]
					if !slices.Contains(tags_pack, proc_tag) {
						tags_pack = append(tags_pack, proc_tag)
					}
				}
			}
			config.Devices[index].TagsPack = tags_pack
		}
	}

	json_devices, err := json.Marshal(config.Devices)
	if err != nil {
		logger.Println("Ошибка формирования списка устройств (Json):", err)
		panic(err)
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
		logger.Println("Ошибка получения StderrPipe:", err)
		panic(err)
	}
	stdout_pipe, err := cmd.StdoutPipe()
	if err != nil {
		logger.Println("Ошибка получения StdoutPipe:", err)
		panic(err)
	}
	if err := cmd.Start(); err != nil {
		logger.Println("Ошибка запуска сборщика:", err)
		panic(err)
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
		logger.Println("Ошибка чтения данных сборщика:", err)
	}

	device_count := len(config.Devices)
	ShutdownCollector(cmd, time.Second*time.Duration(device_count*10))
}
