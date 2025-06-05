package main

import (
	"encoding/json"
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

func FanucDataCollector(device Device, timeout int, global_handle *uint16, running *bool, wait_group *sync.WaitGroup) {
	defer wait_group.Done()
	// extra parameters
	power_on := 1
	stacked_handle := uint16(0)
	var free_handle_error int16
	var handle uint16
	var handle_error int16
	var json_data string
	for *running {
		if stacked_handle != 0 {
			free_handle_error = FreeHandle(&stacked_handle)
			if free_handle_error == 0 || free_handle_error == -8 {
				stacked_handle = 0
				logger.Println("Освобождение дескриптора: успешно")
			}
			time.Sleep(time.Duration(10) * time.Second)
		} else {
			handle, handle_error = GetHandle(device.Address, device.Port, timeout)
			switch handle_error {
			case 0:
				*global_handle = handle
				power_on = 1
				json_data = GetFanucJsonData(&device, &handle, &handle_error)
				OutputFanucData(json_data)
				free_handle_error = FreeHandle(&handle)
				if free_handle_error != 0 && stacked_handle == 0 && handle != 0 {
					stacked_handle = handle
					logger.Printf("Ошибка освобождения дескриптора, handle: %d, error: %d \n", handle, free_handle_error)
				}
			case -16:
				if power_on == 1 {
					logger.Println("Отсутсвует питание устройства (EW_SOCKET: -16)")
					json_data = GetDefaultJsonData(&device, &handle, &handle_error)
					OutputFanucData(json_data)
					power_on = 0
				}
			default:
				logger.Println("Ошибка получения дескриптора, error: ", handle_error)
			}
			time.Sleep(time.Duration(device.DelayMs) * time.Millisecond)
		}
	}
}

func GetDefaultJsonData(device *Device, handle *uint16, handle_error *int16) string {
	tag_map := make(map[string]any)
	// default tags
	tag_map["name"] = device.Name
	tag_map["address"] = device.Address
	tag_map["port"] = device.Port
	if *handle_error == -16 {
		tag_map["power_on"] = 0
	} else {
		tag_map["power_on"] = 1
	}
	json_data, err := json.Marshal(tag_map)
	if err != nil {
		return ""
	}
	return string(json_data)
}

func GetFanucJsonData(device *Device, handle *uint16, handle_error *int16) string {
	tag_map := make(map[string]any)
	// default tags
	tag_map["name"] = device.Name
	tag_map["address"] = device.Address
	tag_map["port"] = device.Port
	if *handle_error == -16 {
		tag_map["power_on"] = 0
	} else {
		tag_map["power_on"] = 1
	}
	// scan tags
	errors := make(map[string]int16)
	for _, tag := range device.TagsPack {
		switch tag {
		case "aut":
			tag_map["aut"], errors["aut"] = GetAut(handle)
		case "run":
			tag_map["run"], errors["run"] = GetRun(handle)
		case "edit":
			tag_map["edit"], errors["edit"] = GetEdit(handle)
		case "g00":
			tag_map["g00"], errors["g00"] = GetG00(handle)
		case "shutdowns":
			tag_map["shutdowns"], errors["shutdowns"] = GetShutdowns(handle)
		case "motion":
			tag_map["motion"], errors["motion"] = GetMotion(handle)
		case "mstb":
			tag_map["mstb"], errors["mstb"] = GetMstb(handle)
		case "load_excess":
			tag_map["load_excess"], errors["load_excess"] = GetLoadExcess(handle)
		case "frame":
			tag_map["frame"], errors["frame"] = GetFrame(handle)
		case "main_prog_number":
			tag_map["main_prog_number"], errors["main_prog_number"] = GetMainProgNum(handle)
		case "sub_prog_number":
			tag_map["sub_prog_number"], errors["sub_prog_number"] = GetSubProgNum(handle)
		case "parts_count":
			tag_map["parts_count"], errors["parts_count"] = GetPartsCount(handle)
		case "tool_number":
			tag_map["tool_number"], errors["tool_number"] = GetToolNumber(handle)
		case "frame_number":
			tag_map["frame_number"], errors["frame_number"] = GetFrameNumber(handle)
		case "feedrate":
			tag_map["feedrate"], errors["feedrate"] = GetFeedRate(handle)
		case "feed_override":
			tag_map["feed_override"], errors["feed_override"] = GetFeedOverride(handle)
		case "jog_override":
			tag_map["jog_override"], errors["jog_override"] = GetJogOverride(handle)
		case "jog_speed":
			tag_map["jog_speed"], errors["jog_speed"] = GetJogSpeed(handle)
		case "current_load":
			tag_map["current_load"], errors["current_load"] = GetServoCurrentLoad(handle)
		case "current_load_percent":
			tag_map["current_load_percent"], errors["current_load_percent"] = GetServoCurrentLoadPercent(handle)
		case "servo_loads":
			tag_map["servo_loads"], errors["servo_loads"] = GetServoLoad(handle)
		case "absolute_positions":
			tag_map["absolute_positions"], errors["absolute_positions"] = GetAbsolutePositions(handle)
		case "machine_positions":
			tag_map["machine_positions"], errors["machine_positions"] = GetMachinePositions(handle)
		case "relative_positions":
			tag_map["relative_positions"], errors["relative_positions"] = GetRelativePositions(handle)
		case "spindle_speed":
			tag_map["spindle_speed"], errors["spindle_speed"] = GetSpindleSpeed(handle)
		case "spindle_param_speed":
			tag_map["spindle_param_speed"], errors["spindle_param_speed"] = GetSpindleSpeedParam(handle)
		case "spindle_motor_speed":
			tag_map["spindle_motor_speed"], errors["spindle_motor_speed"] = GetSpindleMotorSpeed(handle)
		case "spindle_load":
			tag_map["spindle_load"], errors["spindle_load"] = GetSpindleLoad(handle)
		case "spindle_override":
			tag_map["spindle_override"], errors["spindle_override"] = GetSpindleOverride(handle)
		case "emergency":
			tag_map["emergency"], errors["emergency"] = GetEmergency(handle)
		case "alarm":
			tag_map["alarm"], errors["alarm"] = GetAlarm(handle)
		case "axes_number":
			tag_map["axes_number"], errors["axes_number"] = GetCtrlAxesNumber(handle)
		case "spindles_number":
			tag_map["spindles_number"], errors["spindles_number"] = GetCtrlSpindlesNumber(handle)
		case "channels_number":
			tag_map["channels_number"], errors["channels_number"] = GetCtrlPathsNumber(handle)
		case "power_on_time":
			tag_map["power_on_time"], errors["power_on_time"] = GetPowerOnTime(handle)
		case "operation_time":
			tag_map["operation_time"], errors["operation_time"] = GetOperationTime(handle)
		case "cutting_time":
			tag_map["cutting_time"], errors["cutting_time"] = GetCuttingTime(handle)
		case "cycle_time":
			tag_map["cycle_time"], errors["cycle_time"] = GetCycleTime(handle)
		case "series_number":
			tag_map["series_number"], errors["series_number"] = GetSeriesNumber(handle)
		case "version_number":
			tag_map["version_number"], errors["version_number"] = GetVersionNumber(handle)
		case "serial_number":
			tag_map["serial_number"], errors["serial_number"] = GetSerialNumber(handle)
		case "cnc_id":
			tag_map["cnc_id"], errors["cnc_id"] = GetCncId(handle)
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
