package main

/*
#cgo CFLAGS: -I./src
#cgo LDFLAGS: -L./focas -lfwlib32
#include <stdlib.h>
#include "focas/fwlib32.h"
static short get_max_axis()
{
    return MAX_AXIS;
}
static short get_max_spindles()
{
    return MAX_SPINDLE;
}
*/
import "C"

import (
	"fmt"
	"math"
	"strings"
	"unsafe"
)

// Handle functions
func GetHandle(address string, port int, timeout int) (uint16, int16) {
	var handle C.ushort
	c_address := C.CString(address)
	defer C.free(unsafe.Pointer(c_address))
	ret := C.cnc_allclibhndl3(c_address, C.ushort(port), C.long(timeout), &handle)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return uint16(handle), 0
}

func FreeHandle(handle *uint16) int16 {
	return int16(C.cnc_freelibhndl(C.ushort(*handle)))
}

// Mode functions
func GetAut(handle *uint16) (int16, int16) {
	var buf C.ODBST
	ret := C.cnc_statinfo(C.ushort(*handle), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return int16(buf.run), 0
}

func GetRun(handle *uint16) (int16, int16) {
	var buf C.ODBST
	ret := C.cnc_statinfo(C.ushort(*handle), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return int16(buf.run), 0
}

func GetEdit(handle *uint16) (int16, int16) {
	var buf C.ODBST
	ret := C.cnc_statinfo(C.ushort(*handle), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return int16(buf.edit), 0
}

func GetMstb(handle *uint16) (int16, int16) {
	var buf C.ODBST
	ret := C.cnc_statinfo(C.ushort(*handle), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return int16(buf.mstb), 0
}

func GetMotion(handle *uint16) (int16, int16) {
	var buf C.ODBST
	ret := C.cnc_statinfo(C.ushort(*handle), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return int16(buf.motion), 0
}

func GetG00(handle *uint16) (int16, int16) {
	var buf C.ODBMDL
	ret := C.cnc_modal(C.ushort(*handle), C.short(0), C.short(0), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	g_data := (*byte)(unsafe.Pointer(&buf.modal[0]))
	if *g_data == 0 {
		return 1, 0
	}
	return 0, 0
}

func GetShutdowns(handle *uint16) (int16, int16) {
	var length C.ushort = 256
	var blknum C.short
	var buf [256]C.char
	ret := C.cnc_rdexecprog(C.ushort(*handle), &length, &blknum, &buf[0])
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	commands := []string{"M00", "M01", "G04"}
	go_str := C.GoString(&buf[0])
	splitted_str := strings.Split(go_str, "\n")
	for index, cmd := range commands {
		for _, part := range splitted_str {
			if strings.Contains(part, cmd) {
				return int16(index), 0
			}
		}
	}
	return 3, 0
}

func GetLoadExcess(handle *uint16) (int16, int16) {
	result := int16(0)
	num := C.get_max_axis()
	buf_1 := make([]C.ODBSVLOAD, num)
	ret := C.cnc_rdsvmeter(C.ushort(*handle), &num, (*C.ODBSVLOAD)(unsafe.Pointer(&buf_1[0])))
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	for _, data := range buf_1 {
		dec := int16(data.svload.dec)
		value := float64(data.svload.data) * math.Pow(10, -float64(dec))
		if value > 100 {
			result = 1
			break
		}
	}
	num = C.get_max_spindles()
	buf_2 := make([]C.ODBSPLOAD, num)
	ret = C.cnc_rdspmeter(C.ushort(*handle), 0, &num, (*C.ODBSPLOAD)(unsafe.Pointer(&buf_2[0])))
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	for _, data := range buf_2 {
		dec := int16(data.spload.dec)
		value := float64(data.spload.data) * math.Pow(10, -float64(dec))
		if value > 100 {
			if result == 0 {
				return 2, 0
			} else {
				return 3, 0
			}
		}
	}
	return 0, 0
}

// Program functions

func GetMainProgNum(handle *uint16) (int16, int16) {
	var buf C.ODBPRO
	ret := C.cnc_rdprgnum(C.ushort(*handle), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return int16(buf.mdata), 0
}

func GetSubProgNum(handle *uint16) (int16, int16) {
	var buf C.ODBPRO
	ret := C.cnc_rdprgnum(C.ushort(*handle), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return int16(buf.data), 0
}

func GetFrameNumber(handle *uint16) (int64, int16) {
	var buf C.ODBSEQ
	ret := C.cnc_rdseqnum(C.ushort(*handle), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return int64(buf.data), 0
}

func GetFrame(handle *uint16) (string, int16) {
	var length C.ushort = 1024
	var blknum C.short
	var buf [1024]C.char
	ret := C.cnc_rdexecprog(C.ushort(*handle), &length, &blknum, &buf[0])
	if ret != C.EW_OK {
		return "", int16(ret)
	}
	go_str := C.GoString(&buf[0])
	splitted_str := strings.Split(go_str, "\n")
	find_part := "N"
	for _, check_str := range splitted_str {
		if strings.Contains(check_str, find_part) {
			return check_str, 0
		}
	}
	return splitted_str[0], 0
}

func GetPartsCount(handle *uint16) (int64, int16) {
	var buf C.IODBPSD
	ret := C.cnc_rdparam(C.ushort(*handle), 6711, -1, C.short(unsafe.Sizeof(buf)), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	rdata := (*C.REALPRM)(unsafe.Pointer(&buf.u[0]))
	return int64(rdata.prm_val), 0
}

func GetToolNumber(handle *uint16) (int64, int16) {
	var buf C.ODBTLIFE4
	ret := C.cnc_toolnum(C.ushort(*handle), 0, 0, &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return int64(buf.data), 0
}

// Axis functions
func GetAbsolutePositions(handle *uint16) (map[string]float64, int16) {
	result := make(map[string]float64)
	num := C.get_max_axis()
	buf := make([]C.ODBPOS, int(num))
	ret := C.cnc_rdposition(C.ushort(*handle), C.short(0), &num, (*C.ODBPOS)(unsafe.Pointer(&buf[0])))
	if ret != C.EW_OK {
		return result, int16(ret)
	}
	for _, data := range buf {
		name := string(byte(data.abs.name))
		result[name] = float64(data.abs.data) * math.Pow(10, -float64(data.abs.dec))
	}
	return result, 0
}

func GetRelativePositions(handle *uint16) (map[string]float64, int16) {
	result := make(map[string]float64)
	num := C.get_max_axis()
	buf := make([]C.ODBPOS, int(num))
	ret := C.cnc_rdposition(C.ushort(*handle), C.short(0), &num, (*C.ODBPOS)(unsafe.Pointer(&buf[0])))
	if ret != C.EW_OK {
		return result, int16(ret)
	}
	for _, data := range buf {
		name := string(byte(data.rel.name))
		result[name] = float64(data.rel.data) * math.Pow(10, -float64(data.rel.dec))
	}
	return result, 0
}

func GetMachinePositions(handle *uint16) (map[string]float64, int16) {
	result := make(map[string]float64)
	num := C.get_max_axis()
	buf := make([]C.ODBPOS, int(num))
	ret := C.cnc_rdposition(C.ushort(*handle), C.short(0), &num, (*C.ODBPOS)(unsafe.Pointer(&buf[0])))
	if ret != C.EW_OK {
		return result, int16(ret)
	}
	for _, data := range buf {
		name := string(byte(data.mach.name))
		result[name] = float64(data.mach.data) * math.Pow(10, -float64(data.mach.dec))
	}
	return result, 0
}

func GetFeedRate(handle *uint16) (int64, int16) {
	var buf C.ODBACT
	ret := C.cnc_actf(C.ushort(*handle), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return int64(buf.data), 0
}

func GetFeedOverride(handle *uint16) (int16, int16) {
	var buf C.IODBSGNL
	ret := C.cnc_rdopnlsgnl(C.ushort(*handle), C.short(0x20), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return int16(buf.feed_ovrd), 0
}

func GetJogOverride(handle *uint16) (int16, int16) {
	var buf C.IODBSGNL
	ret := C.cnc_rdopnlsgnl(C.ushort(*handle), C.short(0x20), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return int16(buf.jog_ovrd), 0
}

func GetJogSpeed(handle *uint16) (map[string]float64, int16) {
	result := make(map[string]float64)
	num := C.get_max_axis()
	buf := make([]C.ODBAXDT, int(num))
	var types [1]C.short = [1]C.short{2}
	ret := C.cnc_rdaxisdata(C.ushort(*handle), C.short(5), &types[0], C.short(1), &num, (*C.ODBAXDT)(unsafe.Pointer(&buf[0])))
	if ret != C.EW_OK {
		return result, int16(ret)
	}
	var flag int16 = 0
	for _, axis_data := range buf {
		flag = (int16(axis_data.flag) >> 1) & 1
		if flag == 0 {
			name := C.GoString((*C.char)(unsafe.Pointer(&axis_data.name[0])))
			result[name] = float64(axis_data.data) * math.Pow(10, -float64(axis_data.dec))
		}
	}
	return result, 0
}

func GetServoLoad(handle *uint16) (map[string]int64, int16) {
	result := make(map[string]int64)
	num := C.get_max_axis()
	buf := make([]C.ODBSVLOAD, num)
	ret := C.cnc_rdsvmeter(C.ushort(*handle), &num, (*C.ODBSVLOAD)(unsafe.Pointer(&buf[0])))
	if ret != C.EW_OK {
		return result, int16(ret)
	}
	for _, axis_data := range buf {
		name := string(byte(axis_data.svload.name))
		result[name] = int64(axis_data.svload.data)
	}
	return result, 0
}

func GetServoCurrentLoad(handle *uint16) (map[string]float64, int16) {
	result := make(map[string]float64)
	num := C.get_max_axis()
	buf := make([]C.ODBAXDT, int(num))
	var types [1]C.short = [1]C.short{2}
	ret := C.cnc_rdaxisdata(C.ushort(*handle), C.short(2), &types[0], C.short(1), &num, (*C.ODBAXDT)(unsafe.Pointer(&buf[0])))
	if ret != C.EW_OK {
		return result, int16(ret)
	}
	for _, axis_data := range buf {
		name := C.GoString((*C.char)(unsafe.Pointer(&axis_data.name[0])))
		result[name] = float64(axis_data.data) * math.Pow(10, -float64(axis_data.dec))
	}
	return result, 0
}

func GetServoCurrentLoadPercent(handle *uint16) (map[string]int64, int16) {
	result := make(map[string]int64)
	num := C.get_max_axis()
	buf := make([]C.ODBAXDT, int(num))
	var types [1]C.short = [1]C.short{1}
	ret := C.cnc_rdaxisdata(C.ushort(*handle), C.short(2), &types[0], C.short(1), &num, (*C.ODBAXDT)(unsafe.Pointer(&buf[0])))
	if ret != C.EW_OK {
		return result, int16(ret)
	}
	for _, axis_data := range buf {
		name := C.GoString((*C.char)(unsafe.Pointer(&axis_data.name[0])))
		result[name] = int64(axis_data.data)
	}
	return result, 0
}

// Spindle functions

func GetSpindleSpeed(handle *uint16) (float64, int16) {
	var buf C.ODBSPEED
	ret := C.cnc_rdspeed(C.ushort(*handle), C.short(1), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return float64(buf.acts.data) * math.Pow(10, -float64(buf.acts.dec)), 0
}

func GetSpindleSpeedParam(handle *uint16) (map[string]int64, int16) {
	result := make(map[string]int64)
	num := C.get_max_spindles()
	buf := make([]C.ODBAXDT, int(num))
	var types [1]C.short = [1]C.short{2}
	ret := C.cnc_rdaxisdata(C.ushort(*handle), C.short(3), &types[0], C.short(1), &num, (*C.ODBAXDT)(unsafe.Pointer(&buf[0])))
	if ret != C.EW_OK {
		return result, int16(ret)
	}
	for _, axis_data := range buf {
		name := C.GoString((*C.char)(unsafe.Pointer(&axis_data.name[0])))
		result[name] = int64(axis_data.data)
	}
	return result, 0
}

func GetSpindleMotorSpeed(handle *uint16) (map[string]int64, int16) {
	result := make(map[string]int64)
	num := C.get_max_spindles()
	buf := make([]C.ODBSPLOAD, num)
	ret := C.cnc_rdspmeter(C.ushort(*handle), 1, &num, (*C.ODBSPLOAD)(unsafe.Pointer(&buf[0])))
	if ret != C.EW_OK {
		return result, int16(ret)
	}
	for _, spindle_data := range buf {
		name := string(byte(spindle_data.spspeed.name)) + string(byte(spindle_data.spspeed.suff1))
		result[name] = int64(spindle_data.spspeed.data)
	}
	return result, 0
}

func GetSpindleLoad(handle *uint16) (map[string]int64, int16) {
	result := make(map[string]int64)
	num := C.get_max_spindles()
	buf := make([]C.ODBSPLOAD, num)
	ret := C.cnc_rdspmeter(C.ushort(*handle), 0, &num, (*C.ODBSPLOAD)(unsafe.Pointer(&buf[0])))
	if ret != C.EW_OK {
		return result, int16(ret)
	}
	for _, spindle_data := range buf {
		name := string(byte(spindle_data.spload.name)) + string(byte(spindle_data.spload.suff1))
		result[name] = int64(spindle_data.spload.data)
	}
	return result, 0
}

// only 15i function
func GetSpindleOverride(handle *uint16) (int16, int16) {
	var buf C.IODBSGNL
	ret := C.cnc_rdopnlsgnl(C.ushort(*handle), C.short(0x40), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return int16(buf.spdl_ovrd), 0
}

// Alarm functions
func GetEmergency(handle *uint16) (int16, int16) {
	var buf C.ODBST
	ret := C.cnc_statinfo(C.ushort(*handle), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return int16(buf.emergency), 0
}

func GetAlarm(handle *uint16) (int16, int16) {
	var buf C.ODBST
	ret := C.cnc_statinfo(C.ushort(*handle), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return int16(buf.alarm), 0
}

// Operating functions
func GetPowerOnTime(handle *uint16) (int64, int16) {
	var buf C.IODBPSD
	ret := C.cnc_rdparam(C.ushort(*handle), 6750, -1, C.short(unsafe.Sizeof(buf)), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	rdata := (*C.REALPRM)(unsafe.Pointer(&buf.u[0]))
	return int64(rdata.prm_val), 0
}

func GetOperationTime(handle *uint16) (float64, int16) {
	var buf_1 C.IODBPSD
	ret := C.cnc_rdparam(C.ushort(*handle), 6751, -1, C.short(unsafe.Sizeof(buf_1)), &buf_1)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	rdata_1 := (*C.REALPRM)(unsafe.Pointer(&buf_1.u[0]))
	var buf_2 C.IODBPSD
	ret = C.cnc_rdparam(C.ushort(*handle), 6752, -1, C.short(unsafe.Sizeof(buf_2)), &buf_2)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	rdata_2 := (*C.REALPRM)(unsafe.Pointer(&buf_2.u[0]))
	return float64(rdata_2.prm_val)*60 + float64(rdata_1.prm_val)/1000.0, 0
}

func GetCuttingTime(handle *uint16) (float64, int16) {
	var buf_1 C.IODBPSD
	ret := C.cnc_rdparam(C.ushort(*handle), 6753, -1, C.short(unsafe.Sizeof(buf_1)), &buf_1)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	rdata_1 := (*C.REALPRM)(unsafe.Pointer(&buf_1.u[0]))
	var buf_2 C.IODBPSD
	ret = C.cnc_rdparam(C.ushort(*handle), 6754, -1, C.short(unsafe.Sizeof(buf_2)), &buf_2)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	rdata_2 := (*C.REALPRM)(unsafe.Pointer(&buf_2.u[0]))
	return float64(rdata_2.prm_val)*60 + float64(rdata_1.prm_val)/1000.0, 0
}

func GetCycleTime(handle *uint16) (float64, int16) {
	var buf_1 C.IODBPSD
	ret := C.cnc_rdparam(C.ushort(*handle), 6757, -1, C.short(unsafe.Sizeof(buf_1)), &buf_1)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	rdata_1 := (*C.REALPRM)(unsafe.Pointer(&buf_1.u[0]))
	var buf_2 C.IODBPSD
	ret = C.cnc_rdparam(C.ushort(*handle), 6758, -1, C.short(unsafe.Sizeof(buf_2)), &buf_2)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	rdata_2 := (*C.REALPRM)(unsafe.Pointer(&buf_2.u[0]))
	return float64(rdata_2.prm_val)*60 + float64(rdata_1.prm_val)/1000.0, 0
}

func GetSeriesNumber(handle *uint16) (string, int16) {
	var buf C.ODBSYS
	ret := C.cnc_sysinfo(C.ushort(*handle), &buf)
	if ret != C.EW_OK {
		return "", int16(ret)
	}
	return C.GoString((*C.char)(unsafe.Pointer(&buf.series[0]))), 0
}

func GetVersionNumber(handle *uint16) (string, int16) {
	var buf C.ODBSYS
	ret := C.cnc_sysinfo(C.ushort(*handle), &buf)
	if ret != C.EW_OK {
		return "", int16(ret)
	}
	return C.GoString((*C.char)(unsafe.Pointer(&buf.version[0]))), 0
}

func GetCtrlAxesNumber(handle *uint16) (int16, int16) {
	var buf C.ODBSYSEX
	ret := C.cnc_sysinfo_ex(C.ushort(*handle), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return int16(buf.ctrl_axis), 0
}

func GetCtrlSpindlesNumber(handle *uint16) (int16, int16) {
	var buf C.ODBSYSEX
	ret := C.cnc_sysinfo_ex(C.ushort(*handle), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return int16(buf.ctrl_spdl), 0
}

func GetCtrlPathsNumber(handle *uint16) (int16, int16) {
	var buf C.ODBSYSEX
	ret := C.cnc_sysinfo_ex(C.ushort(*handle), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	return int16(buf.ctrl_path), 0
}

func GetSerialNumber(handle *uint16) (int64, int16) {
	var buf C.IODBPSD
	ret := C.cnc_rdparam(C.ushort(*handle), 13151, -1, C.short(unsafe.Sizeof(buf)), &buf)
	if ret != C.EW_OK {
		return 0, int16(ret)
	}
	rdata := (*C.REALPRM)(unsafe.Pointer(&buf.u[0]))
	return int64(rdata.prm_val), 0
}

func GetCncId(handle *uint16) (string, int16) {
	var cnc_ids [4]uint32
	ret := C.cnc_rdcncid(C.ushort(*handle), (*C.ulong)(unsafe.Pointer(&cnc_ids[0])))
	if ret != C.EW_OK {
		return "", int16(ret)
	}
	return fmt.Sprintf("%08x-%08x-%08x-%08x", cnc_ids[0], cnc_ids[1], cnc_ids[2], cnc_ids[3]), 0
}
