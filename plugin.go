package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

type Server struct {
	Status bool `json:"status"`
}

type Device struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
	Series  string `json:"series"`
}

type Config struct {
	CollectorPath string   `json:"collector path"`
	TimeOut       float32  `json:"timeout"`
	Server        Server   `json:"server"`
	Devices       []Device `json:"devices"`
}

type DeviceData struct {
	ProgName string
	ProgNum  int
}

func main() {
	buffer, err := os.ReadFile("plugin.json")
	if err != nil {
		fmt.Println("Не удалось открыть файл")
		return
	}

	var config Config
	err = json.NewDecoder(bytes.NewBuffer(buffer)).Decode(&config)
	if err != nil {
		fmt.Println("Ошибка файла конфигурации: ", err)
		return
	}

	if config.Server.Status {
		inicialize()
		go launch()
	}

	collector_path := config.CollectorPath

	for {
		jsonData, err := json.Marshal(config.Devices)
		if err != nil {
			fmt.Println("Ошибка чтения JSON:", err)
			return
		}

		cmd := exec.Command(collector_path, string(jsonData))

		output, err := cmd.Output()
		if err != nil {
			panic(err)
		}

		fmt.Println(string(output))

		time.Sleep(time.Second * time.Duration(config.TimeOut))
	}
}
