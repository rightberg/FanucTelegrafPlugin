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

type Server struct {
	Status    bool              `json:"status" yaml:"status"`
	Security  map[string]string `yaml:"security"`
	AuthModes []string          `yaml:"auth_modes"`
	MakeCSV   bool              `json:"make csv" yaml:"make_csv"`
}

type Device struct {
	Name    string `json:"name" yaml:"name"`
	Address string `json:"address" yaml:"address"`
	Port    int    `json:"port" yaml:"port"`
	Series  string `json:"series" yaml:"series"`
}

type Config struct {
	CollectorPath string   `json:"collector path" yaml:"collector_path"`
	Interval      float32  `json:"interval" yaml:"interval"`
	Server        Server   `json:"server" yaml:"server"`
	Devices       []Device `json:"devices" yaml:"devices"`
}

type ModeData struct {
	Mode       string `json:"mode" yaml:"mode"`
	RunState   string `json:"run_state" yaml:"run_state"`
	Status     string `json:"status" yaml:"status"`
	Shutdowns  string `json:"shutdowns" yaml:"shutdowns"`
	HightSpeed string `json:"hight_speed" yaml:"hight_speed"`
	AxisMotion string `json:"axis_motion" yaml:"axis_motion"`
	Mstb       string `json:"mstb" yaml:"mstb"`
	LoadExcess string `json:"load_excess" yaml:"load_excess"`
	ModeErr    string `json:"mode_err" yaml:"mode_err"`
}

type ProgramData struct {
	Frame          string `json:"frame" yaml:"frame"`
	MainProgNumber int    `json:"main_prog_number" yaml:"main_prog_number"`
	SubProgNumber  int    `json:"sub_prog_number" yaml:"sub_prog_number"`
	PartsCount     int    `json:"parts_count" yaml:"parts_count"`
	ToolNumber     int    `json:"tool_number" yaml:"tool_number"`
	FrameNumber    int    `json:"frame_number" yaml:"frame_number"`
	PrgErr         string `json:"prg_err" yaml:"prg_err"`
}

type AxesData struct {
	FeedRate           int            `json:"feedrate" yaml:"feed_rate"`
	FeedOverride       int            `json:"feed_override" yaml:"feed_override"`
	JogOverride        float64        `json:"jog_override" yaml:"jog_override"`
	JogSpeed           int            `json:"jog_speed" yaml:"jog_speed"`
	CurrentLoad        float64        `json:"current_load" yaml:"current_load"`
	CurrentLoadPercent float64        `json:"current_load_percent" yaml:"current_load_percent"`
	ServoLoads         map[string]int `json:"servo_loads" yaml:"servo_loads"`
	AxesErr            string         `json:"axes_err" yaml:"axes_err"`
}

type SpindleData struct {
	SpindleSpeed      int            `json:"spindle_speed" yaml:"spindle_speed"`
	SpindleSpeedParam int            `json:"spindle_param_speed" yaml:"spindle_param_speed"`
	SpindleMotorSpeed map[string]int `json:"spindle_motor_speed" yaml:"spindle_motor_speed"`
	SpindleLoad       map[string]int `json:"spindle_load" yaml:"spindle_load"`
	SpindleOverride   int            `json:"spindle_override" yaml:"spindle_override"`
	SpindleErr        string         `json:"spindle_err" yaml:"spindle_err"`
}

type CollectorData struct {
	Device  Device      `json:"device" yaml:"device"`
	Mode    ModeData    `json:"mode_data" yaml:"mode_data"`
	Program ProgramData `json:"program_data" yaml:"program_data"`
	Axes    AxesData    `json:"axes_data" yaml:"axes_data"`
	Spindle SpindleData `json:"spindle_data" yaml:"spindle_data"`
}

type CollectorsData struct {
	Collectors []CollectorData `json:"collectors" yaml:"collectors"`
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
			for index, collector := range col_nodes {
				MakeCSV(GetTagsAtOpcNodes(collector), config.Devices[index].Name, plugin_dir)
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
			UpdateDeviceNodes(&collectors_data)
		}

		fmt.Fprintln(os.Stdout, string(output))
		time.Sleep(time.Second * time.Duration(config.Interval))
	}
}
