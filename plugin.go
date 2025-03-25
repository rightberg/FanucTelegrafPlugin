package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

type Server struct {
	Status  bool `json:"status"`
	MakeCSV bool `json:"make csv"`
}

type Device struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	Series  string `json:"series"`
}

type Config struct {
	CollectorPath string   `json:"collector path"`
	Interval      float32  `json:"interval"`
	Server        Server   `json:"server"`
	Devices       []Device `json:"devices"`
}

type ModeData struct {
	Mode       string `json:"mode"`
	RunState   string `json:"run_state"`
	Status     string `json:"status"`
	Shutdowns  string `json:"shutdowns"`
	HightSpeed string `json:"hight_speed"`
	AxisMotion string `json:"axis_motion"`
	Mstb       string `json:"mstb"`
	LoadExcess string `json:"load_excess"`
	ModeErr    string `json:"mode_err"`
}

type ProgramData struct {
	Frame          string `json:"frame"`
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

type CollectorData struct {
	Device  Device      `json:"device"`
	Mode    ModeData    `json:"mode_data"`
	Program ProgramData `json:"program_data"`
	Axes    AxesData    `json:"axes_data"`
}

type CollectorsData struct {
	Collectors []CollectorData `json:"collectors"`
}

var collectors_data CollectorsData

func main() {
	plugin_path, err := os.Executable()
	if err != nil {
		fmt.Println("Ошибка при определении пути исполняемого файла:", err)
		return
	}

	plugin_dir := filepath.Dir(plugin_path)
	data_path := filepath.Join(plugin_dir, "plugin.json")

	buffer, err := os.ReadFile(data_path)
	if err != nil {
		fmt.Println("Не удалось открыть файл plugin.json:", err)
		return
	}

	var config Config

	err = json.NewDecoder(bytes.NewBuffer(buffer)).Decode(&config)
	if err != nil {
		fmt.Println("Ошибка файла конфигурации: ", err)
		return
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
		jsonData, err := json.Marshal(config.Devices)
		if err != nil {
			fmt.Println("Ошибка чтения JSON:", err)
			return
		}

		cmd := exec.Command(collector_path, string(jsonData))

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
