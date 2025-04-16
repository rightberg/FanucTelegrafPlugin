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
	CollectorPath string   `json:"collector_path" yaml:"collector_path"`
	Interval      float32  `json:"interval" yaml:"interval"`
	Server        Server   `json:"server" yaml:"server"`
	Devices       []Device `json:"devices" yaml:"devices"`
}

type ModeData struct {
	Mode          int16  `json:"mode"`
	RunState      int16  `json:"run_state"`
	Status        int16  `json:"status"`
	Shutdowns     int16  `json:"shutdowns"`
	HightSpeed    int16  `json:"hight_speed"`
	AxisMotion    int16  `json:"axis_motion"`
	Mstb          int16  `json:"mstb" yaml:"mstb"`
	LoadExcess    int64  `json:"load_excess"`
	ModeStr       string `json:"mode_str"`
	RunStateStr   string `json:"run_state_str"`
	StatusStr     string `json:"status_str"`
	ShutdownsStr  string `json:"shutdowns_str"`
	HightSpeedStr string `json:"hight_speed_str"`
	AxisMotionStr string `json:"axis_motion_str"`
	MstbStr       string `json:"mstb_str"`
	LoadExcessStr string `json:"load_excess_str"`

	ModeErrors    []int16  `json:"mode_errors"`
	ModeErrorsStr []string `json:"mode_errors_str"`
}

type ProgramData struct {
	Frame          string `json:"frame"`
	MainProgNumber int16  `json:"main_prog_number"`
	SubProgNumber  int16  `json:"sub_prog_number"`
	PartsCount     int    `json:"parts_count"`
	ToolNumber     int    `json:"tool_number"`
	FrameNumber    int    `json:"frame_number"`

	ProgErrors    []int16  `json:"program_errors"`
	ProgErrorsStr []string `json:"program_errors_str"`
}

type AxesData struct {
	FeedRate           int            `json:"feedrate"`
	FeedOverride       int            `json:"feed_override"`
	JogOverride        float64        `json:"jog_override"`
	JogSpeed           int            `json:"jog_speed"`
	CurrentLoad        float64        `json:"current_load"`
	CurrentLoadPercent float64        `json:"current_load_percent"`
	ServoLoads         map[string]int `json:"servo_loads"`

	AxesErrors    []int16  `json:"axes_errors"`
	AxesErrorsStr []string `json:"axes_errors_str"`
}

type SpindleData struct {
	SpindleSpeed      int            `json:"spindle_speed"`
	SpindleSpeedParam int            `json:"spindle_param_speed"`
	SpindleMotorSpeed map[string]int `json:"spindle_motor_speed"`
	SpindleLoad       map[string]int `json:"spindle_load"`
	SpindleOverride   int16          `json:"spindle_override"`

	SpindleErrors    []int16  `json:"spindle_errors"`
	SpindleErrorsStr []string `json:"spindle_errors_str"`
}

type AlarmData struct {
	Emergency   int16 `json:"emergency"`
	AlarmStatus int16 `json:"alarm_status"`

	EmergencyStr   string `json:"emergency_str"`
	AlarmStatusStr string `json:"alarm_status_str"`

	AlarmErrors    []int16  `json:"alarm_errors"`
	AlarmErrorsStr []string `json:"alarm_errors_str"`
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

type TableRow struct {
	Name  string
	Value float64
}

var collectors_data CollectorsData
var config Config
var plugin_dir string

func main() {
	plugin_path, err := os.Executable()
	if err != nil {
		fmt.Println("Ошибка при определении пути исполняемого файла:", err)
		return
	}

	plugin_dir = filepath.Dir(plugin_path)
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

		output, err := cmd.CombinedOutput()
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
