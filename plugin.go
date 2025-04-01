package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type ImportEndpoint struct {
	Endpoint string `yaml:"endpoint"`
	Port     int    `yaml:"server_port"`
}

type Server struct {
	Status    bool              `yaml:"status"`
	MakeCert  bool              `yaml:"make_cert"`
	MakeCSV   bool              `yaml:"make_csv"`
	AuthModes []string          `yaml:"auth_modes"`
	Endpoints []ImportEndpoint  `yaml:"endpoints"`
	Security  map[string]string `yaml:"security"`
}

type Device struct {
	Name    string `json:"name" yaml:"name"`
	Address string `json:"address" yaml:"address"`
	Series  string `json:"series" yaml:"series"`
	Port    int    `json:"port" yaml:"port"`
}

type Config struct {
	CollectorPath string   `json:"collector path" yaml:"collector_path"`
	Interval      float32  `json:"interval" yaml:"interval"`
	Server        Server   `json:"server" yaml:"server"`
	Devices       []Device `json:"devices" yaml:"devices"`
}

type ModeData struct {
	Mode       string `json:"mode"`
	RunState   string `json:"run_state"`
	Status     string `json:"status"`
	Shutdowns  string `json:"shutdowns"`
	HightSpeed string `json:"hight_speed"`
	AxisMotion string `json:"axis_motion"`
	Mstb       string `json:"mstb" yaml:"mstb"`
	LoadExcess string `json:"load_excess"`
	ModeErr    string `json:"mode_err"`
}

type ProgramData struct {
	Frame          string `json:"frame" yaml:"frame"`
	MainProgNumber int    `json:"main_prog_number"`
	SubProgNumber  int    `json:"sub_prog_number"`
	PartsCount     int    `json:"parts_count"`
	ToolNumber     int    `json:"tool_number"`
	FrameNumber    int    `json:"frame_number"`
	PrgErr         string `json:"prg_err"`
}

type AxesData struct {
	FeedRate           int            `json:"feedrate"`
	FeedOverride       int            `json:"feed_override"`
	JogOverride        float64        `json:"jog_override"`
	JogSpeed           int            `json:"jog_speed"`
	CurrentLoad        float64        `json:"current_load"`
	CurrentLoadPercent float64        `json:"current_load_percent"`
	ServoLoads         map[string]int `json:"servo_loads"`
	AxesErr            string         `json:"axes_err"`
}

type SpindleData struct {
	SpindleSpeed      int            `json:"spindle_speed"`
	SpindleSpeedParam int            `json:"spindle_param_speed"`
	SpindleMotorSpeed map[string]int `json:"spindle_motor_speed"`
	SpindleLoad       map[string]int `json:"spindle_load"`
	SpindleOverride   int            `json:"spindle_override"`
	SpindleErr        string         `json:"spindle_err"`
}

type AlarmData struct {
	Emergency   string `json:"emergency"`
	AlarmStatus string `json:"alarm_status"`
	AlarmErr    string `json:"alarm_err"`
}

type CollectorData struct {
	Device  Device      `json:"device"`
	Mode    ModeData    `json:"mode_data"`
	Program ProgramData `json:"program_data"`
	Axes    AxesData    `json:"axes_data"`
	Spindle SpindleData `json:"spindle_data"`
	Alarm   AlarmData   `json:"alarm_data"`
}

type CollectorsData struct {
	Collectors []CollectorData `json:"collectors"`
}

var collectors_data CollectorsData
var config Config

func main() {
	plugin_path, err := os.Executable()
	if err != nil {
		fmt.Println("Ошибка при определении пути исполняемого файла:", err)
		return
	}

	plugin_dir := filepath.Dir(plugin_path)
	data_path := filepath.Join(plugin_dir, "plugin.conf")
	fileContent, err := os.ReadFile(data_path)
	if err != nil {
		log.Fatalf("Ошибка чтения файла: %v", err)
	}

	err = yaml.Unmarshal(fileContent, &config)
	if err != nil {
		log.Fatalf("Ошибка парсинга YAML: %v", err)
	}

	for index := range config.Devices {
		str_index := strconv.Itoa(index)
		if config.Devices[index].Name == "" {
			config.Devices[index].Name = "Device " + str_index
		}
	}

	if config.Server.Status {
		device_count := len(config.Devices)
		collectors_data.Collectors = make([]CollectorData, device_count)
		for index := range collectors_data.Collectors {
			collectors_data.Collectors[index].Device.Name = config.Devices[index].Name
		}
		inicialize()
		if config.Server.MakeCSV {
			for index := range device_addresses {
				device_name := config.Devices[index].Name
				MakeCSV(GetTagsAtOpcNodes(device_name), device_name, plugin_dir)
			}
		}
		go start()
	}

	collector_path := config.CollectorPath
	if collector_path == "" {
		collector_path = filepath.Join(plugin_dir, "collector", "collector.exe")
	}

	for {
		json_data, err := json.Marshal(config.Devices)
		if err != nil {
			fmt.Println("Ошибка чтения JSON:", err)
			return
		}

		cmd := exec.Command(collector_path, string(json_data))

		output, err := cmd.Output()
		if err != nil {
			fmt.Printf("Ошибка сборщика: %s\n", err.Error())
			panic(err)
		}

		dec_err := json.Unmarshal([]byte(output), &collectors_data)
		if dec_err != nil {
			fmt.Println("Ошибка декодирования JSON:", dec_err)
			return
		}

		if config.Server.Status {
			UpdateDeviceNodes(collectors_data.Collectors)
		}

		fmt.Fprintln(os.Stdout, string(output))
		time.Sleep(time.Second * time.Duration(config.Interval))
	}
}
