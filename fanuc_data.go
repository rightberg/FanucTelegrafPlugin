package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

func FreeAllHandles(handles []uint16) {
	var non_free_handles []uint16
	for index := range handles {
		free_handle_error := FreeHandle(&handles[index])
		if free_handle_error != 0 && free_handle_error != -8 {
			logger.Println("Неосвобожденный дескриптор: ", handles[index])
			non_free_handles = append(non_free_handles, handles[index])
		}
	}
	if len(non_free_handles) == 0 {
		return
	}

	json_data, err := json.Marshal(non_free_handles)
	if err != nil {
		logger.Println("Ошибка преобразования данных (non_free_handles) в json", err)
		return
	}

	err = os.WriteFile("non_free_handles.json", json_data, 0644)
	if err != nil {
		logger.Println("Ошибка записи json-данных в файл:", err)
	}
}

func TryFreeExtraHandles(file_dir string) {
	file_path := filepath.Join(file_dir, "non_free_handles.json")
	if _, err := os.Stat(file_path); os.IsNotExist(err) {
		return
	}
	file_content, err := os.ReadFile(file_path)
	if err != nil {
		logger.Println("Ошибка чтения файла: ", err)
		return
	}
	var non_free_handles []uint16
	err = json.Unmarshal(file_content, &non_free_handles)
	if err != nil {
		logger.Println("Ошибка чтения non_free_handles.json: ", err)
		return
	}
	for index := range non_free_handles {
		result := true
		for result {
			free_handle_error := FreeHandle(&non_free_handles[index])
			if free_handle_error == 0 || free_handle_error == -8 {
				result = false
				break
			}
			time.Sleep(time.Duration(10) * time.Second)
		}
	}
	if len(non_free_handles) > 0 {
		logger.Println("Освобождение занятых дескрипторы: успешно")
		err = os.Remove(file_path)
		if err != nil && !os.IsNotExist(err) {
			logger.Println("Ошибка удаления файла: non_free_handles.json", err)
		}
	}
}

func FormatAddress(ip string, port int) string {
	parsed_ip := net.ParseIP(ip)
	if parsed_ip == nil {
		return fmt.Sprintf("%s:%d", ip, port)
	}
	// IPv6
	if parsed_ip.To4() == nil {
		return fmt.Sprintf("[%s]:%d", ip, port)
	}
	// IPv4
	return fmt.Sprintf("%s:%d", ip, port)
}

func IsConnectAlive(ip string, port int, timeout time.Duration, running *bool) bool {
	done := make(chan bool, 1)

	go func() {
		conn, err := net.DialTimeout("tcp", FormatAddress(ip, port), timeout)
		if err != nil {
			done <- false
			return
		}
		conn.Close()
		done <- true
	}()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case result := <-done:
			return result
		case <-ticker.C:
			if !*running {
				return false
			}
		}
	}
}

func StartDataCollector(device Device, timeout int, global_handle *uint16, running *bool, wait_group *sync.WaitGroup) {
	defer wait_group.Done()
	for *running {
		var local_wg sync.WaitGroup
		local_wg.Add(1)
		go func() {
			defer local_wg.Done()
			DataCollector(device, timeout, global_handle, running)
		}()
		local_wg.Wait()
		for i := 0; i < 100; i++ {
			if !*running {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func DataCollector(device Device, timeout int, global_handle *uint16, running *bool) {
	var handle uint16 = 0
	var handle_error int16 = 0
	var json_data string
	reconnect_counter := 0
	max_reconnect := 5
	// free handle
	defer func() {
		if handle != 0 {
			free_handle_error := FreeHandle(&handle)
			if free_handle_error != 0 && free_handle_error != -8 {
				logger.Printf("Ошибка освобождения дескриптора %s, error: %d", device.Name, free_handle_error)
			} else {
				*global_handle = 0
			}
		}
	}()
	connect_count := 0
	max_connect := 5
	get_handle_count := 0
	max_get_handle := 5
	// pull default data
	OutputFanucData(GetPowerOffData(&device))
	// try connect to device
	for connect_count <= max_connect {
		connect_count++
		if IsConnectAlive(device.Address, device.Port, 10*time.Second, running) {
			connect_count = 0
			break
		}
		if connect_count >= max_connect {
			logger.Printf("Устройство %s недоступно проверьте питание и параметры TCP соединения \n", device.Name)
			OutputFanucData(GetPowerOffData(&device))
			return
		}
	}
	//try free handle
	handle = *global_handle
	if handle != 0 {
		free_handle_error := FreeHandle(&handle)
		if free_handle_error != 0 && free_handle_error != -8 {
			logger.Printf("Ошибка освобождения дескриптора %s, error: %d", device.Name, free_handle_error)
		} else {
			*global_handle = 0
			handle = 0
		}
	}
	// try get handle
	for get_handle_count <= max_get_handle {
		get_handle_count++
		handle, handle_error = GetHandleWithTimeout(device.Address, device.Port, timeout)
		if handle_error == 0 {
			get_handle_count = 0
			*global_handle = handle
			break
		}
		if get_handle_count >= max_get_handle {
			logger.Println("Ошибка получения дескриптора, error: ", handle_error)
			OutputFanucData(GetPowerOffData(&device))
			return
		}
	}
	// collect data
	protocol_error := false
	for *running {
		if !IsConnectAlive(device.Address, device.Port, 10*time.Second, running) {
			reconnect_counter++
			if reconnect_counter >= max_reconnect {
				OutputFanucData(GetPowerOffData(&device))
				logger.Println("Попытка перезапустить поток, device: ", device.Name)
				return
			}
			continue
		}
		json_data = GetFanucJsonData(&device, &handle, &protocol_error)
		OutputFanucData(json_data)
		if protocol_error {
			reconnect_counter++
			if reconnect_counter >= max_reconnect {
				OutputFanucData(GetPowerOffData(&device))
				logger.Println("Попытка перезапустить поток, device: ", device.Name)
				return
			}
		}
		time.Sleep(time.Duration(device.DelayMs) * time.Millisecond)
	}
}

func GetPowerOffData(device *Device) string {
	tag_map := make(map[string]any)
	// default tags
	tag_map["name"] = device.Name
	tag_map["address"] = device.Address
	tag_map["port"] = device.Port
	tag_map["power_on"] = 0
	json_data, err := json.Marshal(tag_map)
	if err != nil {
		return ""
	}
	return string(json_data)
}

func GetFanucJsonData(device *Device, handle *uint16, protocol_error *bool) string {
	tag_map := make(map[string]any)
	// default tags
	tag_map["name"] = device.Name
	tag_map["address"] = device.Address
	tag_map["port"] = device.Port
	tag_map["power_on"] = 1
	// scan tags
	*protocol_error = false
	errors := make(map[string]int16)
	for _, tag := range device.TagsPack {
		switch tag {
		case "aut":
			tag_map[tag], errors[tag] = GetAut(handle)
		case "run":
			tag_map[tag], errors[tag] = GetRun(handle)
		case "edit":
			tag_map[tag], errors[tag] = GetEdit(handle)
		case "g00":
			tag_map[tag], errors[tag] = GetG00(handle)
		case "shutdowns":
			tag_map[tag], errors[tag] = GetShutdowns(handle)
		case "motion":
			tag_map[tag], errors[tag] = GetMotion(handle)
		case "mstb":
			tag_map[tag], errors[tag] = GetMstb(handle)
		case "load_excess":
			tag_map[tag], errors[tag] = GetLoadExcess(handle)
		case "frame":
			tag_map[tag], errors[tag] = GetFrame(handle)
		case "main_prog_number":
			tag_map[tag], errors[tag] = GetMainProgNum(handle)
		case "sub_prog_number":
			tag_map[tag], errors[tag] = GetSubProgNum(handle)
		case "parts_count":
			tag_map[tag], errors[tag] = GetPartsCount(handle)
		case "tool_number":
			tag_map[tag], errors[tag] = GetToolNumber(handle)
		case "frame_number":
			tag_map[tag], errors[tag] = GetFrameNumber(handle)
		case "feedrate":
			tag_map[tag], errors[tag] = GetFeedRate(handle)
		case "feedrate_prg":
			tag_map[tag], errors[tag] = GetFeedRateParam1(handle)
		case "feedrate_note":
			tag_map[tag], errors[tag] = GetFeedRateParam2(handle)
		case "feed_override":
			tag_map[tag], errors[tag] = GetFeedOverride(handle)
		case "jog_override":
			tag_map[tag], errors[tag] = GetJogOverride(handle)
		case "jog_speed":
			tag_map[tag], errors[tag] = GetJogSpeed(handle)
		case "current_load":
			tag_map[tag], errors[tag] = GetServoCurrentLoad(handle)
		case "current_load_percent":
			tag_map[tag], errors[tag] = GetServoCurrentLoadPercent(handle)
		case "servo_loads":
			tag_map[tag], errors[tag] = GetServoLoad(handle)
		case "absolute_positions":
			tag_map[tag], errors[tag] = GetAbsolutePositions(handle)
		case "machine_positions":
			tag_map[tag], errors[tag] = GetMachinePositions(handle)
		case "relative_positions":
			tag_map[tag], errors[tag] = GetRelativePositions(handle)
		case "spindle_speed":
			tag_map[tag], errors[tag] = GetSpindleSpeed(handle)
		case "spindle_param_speed":
			tag_map[tag], errors[tag] = GetSpindleSpeedParam(handle)
		case "spindle_motor_speed":
			tag_map[tag], errors[tag] = GetSpindleMotorSpeed(handle)
		case "spindle_load":
			tag_map[tag], errors[tag] = GetSpindleLoad(handle)
		case "spindle_override":
			tag_map[tag], errors[tag] = GetSpindleOverride(handle)
		case "emergency":
			tag_map[tag], errors[tag] = GetEmergency(handle)
		case "alarm":
			tag_map[tag], errors[tag] = GetAlarm(handle)
		case "axes_number":
			tag_map[tag], errors[tag] = GetCtrlAxesNumber(handle)
		case "spindles_number":
			tag_map[tag], errors[tag] = GetCtrlSpindlesNumber(handle)
		case "channels_number":
			tag_map[tag], errors[tag] = GetCtrlPathsNumber(handle)
		case "power_on_time":
			tag_map[tag], errors[tag] = GetPowerOnTime(handle)
		case "operation_time":
			tag_map[tag], errors[tag] = GetOperationTime(handle)
		case "cutting_time":
			tag_map[tag], errors[tag] = GetCuttingTime(handle)
		case "cycle_time":
			tag_map[tag], errors[tag] = GetCycleTime(handle)
		case "series_number":
			tag_map[tag], errors[tag] = GetSeriesNumber(handle)
		case "version_number":
			tag_map[tag], errors[tag] = GetVersionNumber(handle)
		case "serial_number":
			tag_map[tag], errors[tag] = GetSerialNumber(handle)
		case "cnc_id":
			tag_map[tag], errors[tag] = GetCncId(handle)
		}
		if error_code, ok := errors[tag]; ok && (error_code == -16 || error_code == -8) {
			*protocol_error = true
			break
		}
	}
	// clear error data
	for tag_name, error_code := range errors {
		if error_code != 0 {
			delete(tag_map, tag_name)
		}
	}
	// check errors tag
	if slices.Contains(device.TagsPack, "errors") {
		tag_map["errors"] = errors
	}
	// make json data
	json_data, err := json.Marshal(tag_map)
	if err != nil {
		return "{}"
	}
	return string(json_data)
}
