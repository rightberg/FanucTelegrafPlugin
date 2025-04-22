package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"syscall"
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
	Name    string `json:"name" yaml:"name"`
	Address string `json:"address" yaml:"address"`
	Series  string `json:"series" yaml:"series"`
	Port    int    `json:"port" yaml:"port"`
}

type Config struct {
	Interval float32  `json:"interval" yaml:"interval"`
	Server   Server   `json:"server" yaml:"server"`
	Devices  []Device `json:"devices" yaml:"devices"`
}

type FanucData struct {
	// device data
	Name    string `json:"name"`
	Address string `json:"address"`
	Series  string `json:"series"`
	Port    int    `json:"port"`
	// mode data
	Mode       int16 `json:"mode"`
	RunState   int16 `json:"run_state"`
	Status     int16 `json:"status"`
	Shutdowns  int16 `json:"shutdowns"`
	HightSpeed int16 `json:"hight_speed"`
	AxisMotion int16 `json:"axis_motion"`
	Mstb       int16 `json:"mstb"`
	LoadExcess int64 `json:"load_excess"`
	// program data
	Frame          string `json:"frame"`
	MainProgNumber int16  `json:"main_prog_number"`
	SubProgNumber  int16  `json:"sub_prog_number"`
	PartsCount     int    `json:"parts_count"`
	ToolNumber     int64  `json:"tool_number"`
	FrameNumber    int64  `json:"frame_number"`
	// axes data
	JogOverride        int16          `json:"jog_override"`
	FeedOverride       int16          `json:"feed_override"`
	Feedrate           int64          `json:"feedrate"`
	JogSpeed           int64          `json:"jog_speed"`
	CurrentLoad        float32        `json:"current_load"`
	CurrentLoadPercent float32        `json:"current_load_percent"`
	ServoLoads         map[string]int `json:"servo_loads"`
	// spindle data
	SpindleOverride   int16          `json:"spindle_override"`
	SpindleSpeed      int64          `json:"spindle_speed"`
	SpindleParamSpeed int64          `json:"spindle_param_speed"`
	SpindleMotorSpeed map[string]int `json:"spindle_motor_speed"`
	SpindleLoad       map[string]int `json:"spindle_load"`
	// alarm data
	Emergency   int16 `json:"emergency"`
	AlarmStatus int16 `json:"alarm_status"`
	// error data
	Errors    []int16  `json:"errors"`
	ErrorsStr []string `json:"errors_str"`
}

var config Config
var plugin_dir string

var log_buf bytes.Buffer
var logger *log.Logger

func main() {
	multi_writer := io.MultiWriter(os.Stdout, &log_buf)
	logger = log.New(multi_writer, "Plugin: ", log.Ldate|log.Ltime|log.Lshortfile)
	defer func() {
		shutdown_path := filepath.Join(plugin_dir, "plugin.log")
		if err := os.WriteFile(shutdown_path, log_buf.Bytes(), 0644); err != nil {
			log.Println("Ошибка записи файла shutdown.log:", err)
		}
	}()
	defer func() {
		if r := recover(); r != nil {
			logger.Println("Panic:", r)
			trace := debug.Stack()
			logger.Println(string(trace))
			crash_path := filepath.Join(plugin_dir, "crash.log")
			if err := os.WriteFile(crash_path, log_buf.Bytes(), 0644); err != nil {
				log.Println("Ошибка записи файла crash.log:", err)
			}
			os.Exit(1)
		}
	}()

	plugin_path, err := os.Executable()
	if err != nil {
		logger.Println("Ошибка при определении пути исполняемого файла:", err)
		panic(err)
	}

	plugin_dir = filepath.Dir(plugin_path)
	data_path := filepath.Join(plugin_dir, "plugin.conf")
	fileContent, err := os.ReadFile(data_path)
	if err != nil {
		logger.Println("Ошибка чтения файла:", err)
		panic(err)
	}

	err = yaml.Unmarshal(fileContent, &config)
	if err != nil {
		logger.Println("Ошибка чтения plugin.conf (yaml):", err)
		panic(err)
	}

	for index := range config.Devices {
		str_index := strconv.Itoa(index)
		if config.Devices[index].Name == "" {
			config.Devices[index].Name = "Device " + str_index
		}
	}

	if config.Server.Status {
		inicialize()
		go start()
	}

	json_devices, err := json.Marshal(config.Devices)
	if err != nil {
		logger.Println("Ошибка формирования списка устройств (Json):", err)
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	collector_path := filepath.Join(plugin_dir, "collector", "collector.exe")
	cmd := exec.CommandContext(ctx, collector_path, string(json_devices))
	AddProcessToGroup(cmd)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		logger.Println("Ошибка получения StdoutPipe:", err)
		panic(err)
	}
	if err := cmd.Start(); err != nil {
		logger.Println("Ошибка запуска сборщика:", err)
		panic(err)
	}

	// закрытие дочернего процесса
	if runtime.GOOS == "windows" {
		AddWinJobObject(cmd)
	}
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		s := <-sigc
		fmt.Println("try close sub programs")
		ShutdownChildProcess(cmd, cancel, s)
	}()

	var lastReceivedTime time.Time
	scanner := bufio.NewScanner(stdoutPipe)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		go func() {
			var data FanucData
			err := json.Unmarshal([]byte(line), &data)
			if err == nil && config.Server.Status {
				UpdateCollector(data)
			}
			fmt.Fprintln(os.Stdout, line)
		}()

		currentTime := time.Now()
		if !lastReceivedTime.IsZero() {
			interval := currentTime.Sub(lastReceivedTime)
			logger.Printf("Интервал между сообщениями: %s\n", interval)
		} else {
			logger.Println("Первый пакет получен")
		}
		lastReceivedTime = currentTime
	}
	if err := scanner.Err(); err != nil {
		logger.Println("Ошибка чтения данных сборщика:", err)
	}
}
