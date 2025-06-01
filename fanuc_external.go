package main

/*
#cgo CFLAGS: -I.
#cgo LDFLAGS: -L. -lcollector
#cgo CXXFLAGS: -std=c++20
#include "FanucExternal.h"
#include <stdlib.h>
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
	"unsafe"
)

var handles []int
var running = true

func OutputFanucJsonData(json_data string) {
	if json.Valid([]byte(json_data)) {
		if config.Server.Status {
			UpdateCollector(json_data)
		}
		fmt.Fprintln(os.Stdout, json_data)
	}
}

func FreeAllHandles() {
	for handle := range handles {
		free_handle_error := int16(C.FreeHandleExternal(C.ushort(handle)))
		if free_handle_error != 0 {
			logger.Println("Неосвобожденный дескриптор: ", handle)
		}
	}
}

func EndPlugin() {
	logger.Println("Завершение плагина")
	running = false
	time.Sleep(3 * time.Second)
	FreeAllHandles()
}

func FanucDataCollector(device Device, timeout int, running *bool, handle *int, wait_group *sync.WaitGroup) {
	defer wait_group.Done()
	//преобразование device в json
	json_device, err := json.Marshal(device)
	if err != nil {
		logger.Panicf("Ошибка преобразования устройства %s в json-строку: %v", device.Name, err)
	}
	//дополнительные переменные
	stacked_timeout := 10
	power_on := 1
	stacked_handle := int16(0)
	free_handle_error := int16(0)
	var _handle C.UShortDataEx
	c_json_device := C.CString(string(json_device))
	defer C.free(unsafe.Pointer(c_json_device))
	for *running {
		if stacked_handle != 0 {
			free_handle_error = int16(C.FreeHandleExternal(C.ushort(stacked_handle)))
			if free_handle_error == 0 || free_handle_error == -8 {
				stacked_handle = 0
				logger.Println("Освобождение дескриптора: успешно")
			}
			time.Sleep(time.Duration(stacked_timeout) * time.Second)
		} else {
			c_address := C.CString(device.Address)
			_handle = C.GetHandleExternal(c_address, C.int(device.Port), C.int(timeout))
			C.free(unsafe.Pointer(c_address))
			if int(_handle.error) == 0 {
				power_on = 1
				*handle = int(_handle.data)
				c_json_data := C.GetFanucJsonDataExternal(c_json_device, C.ushort(_handle.data), C.short(_handle.error))
				go_json_data := C.GoString(c_json_data)
				C.free(unsafe.Pointer(c_json_data))
				OutputFanucJsonData(go_json_data)
				free_handle_error = int16(C.FreeHandleExternal(_handle.data))
				if free_handle_error != 0 && stacked_handle == 0 && _handle.data != 0 {
					stacked_handle = int16(_handle.data)
					logger.Printf("Ошибка освобождения дескриптора, handle: %d, error: %d \n", _handle.data, free_handle_error)
				}
			} else if int(_handle.error) == -16 {
				if power_on == 1 {
					logger.Println("Отсутсвует питание устройства (EW_SOCKET: -16)")
					c_json_data := C.GetFanucJsonDataExternal(c_json_device, C.ushort(_handle.data), C.short(_handle.error))
					go_json_data := C.GoString(c_json_data)
					C.free(unsafe.Pointer(c_json_data))
					OutputFanucJsonData(go_json_data)
					power_on = 0
				}
			} else {
				logger.Println("Ошибка получения дескриптора, error: ", _handle.error)
			}
			time.Sleep(time.Duration(device.DelayMs) * time.Millisecond)
		}
	}
}
