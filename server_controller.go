package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/gopcua/opcua/debug"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/server"
	"github.com/gopcua/opcua/ua"
)

var (
	certfile = flag.String("cert", "cert.pem", "Path to certificate file")
	keyfile  = flag.String("key", "key.pem", "Path to PEM Private Key file")
)

var available_policies = []string{
	"None",
	"Basic128Rsa15",
	"Basic256",
	"Basic256Sha256",
	"Aes128_Sha256_RsaOaep",
	"Aes128_Sha256_RsaOaep",
}

var available_sec_modes = []string{
	"None",
	"Sign",
	"SignAndEncrypt",
}

var available_auth_modes = []string{
	"Anonymous",
	"Username",
	"Certificate",
}

var _server *server.Server
var fanuc_ns = int(1)
var device_map map[string]string

type Logger int

func (l Logger) Debug(msg string, args ...any) {
	if l < 0 {
		logger.Printf("Server Debug: "+msg, args...)
	}
}

func (l Logger) Info(msg string, args ...any) {
	if l < 1 {
		logger.Printf("Server Info: "+msg, args...)
	}
}

func (l Logger) Warn(msg string, args ...any) {
	if l < 2 {
		logger.Printf("Server Warn: "+msg, args...)
	}
}

func (l Logger) Error(msg string, args ...any) {
	if l < 3 {
		logger.Printf("Server Error: "+msg, args...)
	}
}

func MapToStr(map_data map[string]int) string {
	var merge []string
	for k, v := range map_data {
		merge = append(merge, fmt.Sprintf("%s: %d", k, v))
	}
	res := strings.Join(merge, ", ")
	return res
}

func SeparateMap(data map[string]int) ([]string, []int64) {
	keys := make([]string, 0, len(data))
	values := make([]int64, 0, len(data))
	for k, v := range data {
		keys = append(keys, k)
		values = append(values, int64(v))
	}
	return keys, values
}

func MapKeys(data map[string]int) []string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	return keys
}

func MapValues(data map[string]int) []int64 {
	values := make([]int64, 0, len(data))
	for _, v := range data {
		values = append(values, int64(v))
	}
	return values
}

func GetMapValue(m map[string]int, key string) int {
	if value, exists := m[key]; exists {
		return value
	}
	return 0
}

func GetDeviceNodes(device_name string) []*server.Node {
	var result []*server.Node
	addresses := []string{
		"/name",
		"/address",
		"/port",
		"/series",
		"/aut",
		"/run",
		"/edit",
		"/shutdowns",
		"/hight_speed",
		"/motion",
		"/mstb",
		"/load_excess",
		"/frame",
		"/main_prog_number",
		"/sub_prog_number",
		"/parts_count",
		"/tool_number",
		"/frame_number",
		"/feedrate",
		"/feed_override",
		"/jog_override",
		"/jog_speed",
		"/current_load",
		"/current_load_percent",
		"/servo_load_x",
		"/servo_load_z",
		"/absolute_position_x",
		"/absolute_position_z",
		"/machine_position_x",
		"/machine_position_z",
		"/relative_position_x",
		"/relative_position_z",
		"/spindle_speed",
		"/spindle_param_speed",
		"/spindle_override",
		"/spindle_load_names",
		"/spindle_motor_speed_s",
		"/spindle_load",
		"/emergency",
		"/alarm",
		"/errors",
		"/errors_str",
		"/power_on_time",
		"/operation_time",
		"/cutting_time",
		"/cycle_time",
		"/series_number",
		"/version_number",
	}
	node_ns := GetNodeNamespace(_server, fanuc_ns)
	if node_ns != nil {
		for _, address := range addresses {
			device_address := fmt.Sprintf("ns=%d;s=%s", fanuc_ns, device_name)
			node := GetNodeAtAddress(node_ns, device_address+address)
			if node != nil {
				result = append(result, node)
			}
		}
	}
	return result
}

func UpdateCollector(collector FanucData) {
	node_ns := GetNodeNamespace(_server, fanuc_ns)
	if node_ns == nil {
		logger.Println("UPD некорректный node_ns")
		return
	}
	device_address, exists := device_map[collector.Name]
	if !exists {
		logger.Println("UPD устройство отсутсвует")
		return
	}
	// Device data
	UpdateNodeValueAtAddress(node_ns, device_address+"/name", string(collector.Name))
	UpdateNodeValueAtAddress(node_ns, device_address+"/address", string(collector.Address))
	UpdateNodeValueAtAddress(node_ns, device_address+"/port", int64(collector.Port))
	UpdateNodeValueAtAddress(node_ns, device_address+"/series", string(collector.Series))
	// Mode data
	UpdateNodeValueAtAddress(node_ns, device_address+"/aut", int16(collector.Aut))
	UpdateNodeValueAtAddress(node_ns, device_address+"/run", int16(collector.Run))
	UpdateNodeValueAtAddress(node_ns, device_address+"/edit", int16(collector.Edit))
	UpdateNodeValueAtAddress(node_ns, device_address+"/shutdowns", int16(collector.Shutdowns))
	UpdateNodeValueAtAddress(node_ns, device_address+"/hight_speed", int16(collector.HightSpeed))
	UpdateNodeValueAtAddress(node_ns, device_address+"/motion", int16(collector.Motion))
	UpdateNodeValueAtAddress(node_ns, device_address+"/mstb", int16(collector.Mstb))
	UpdateNodeValueAtAddress(node_ns, device_address+"/load_excess", int64(collector.LoadExcess))
	// Program data
	UpdateNodeValueAtAddress(node_ns, device_address+"/frame", string(collector.Frame))
	UpdateNodeValueAtAddress(node_ns, device_address+"/main_prog_number", int16(collector.MainProgNumber))
	UpdateNodeValueAtAddress(node_ns, device_address+"/sub_prog_number", int16(collector.SubProgNumber))
	UpdateNodeValueAtAddress(node_ns, device_address+"/parts_count", int64(collector.PartsCount))
	UpdateNodeValueAtAddress(node_ns, device_address+"/tool_number", int64(collector.ToolNumber))
	UpdateNodeValueAtAddress(node_ns, device_address+"/frame_number", int64(collector.FrameNumber))
	// Axes data
	UpdateNodeValueAtAddress(node_ns, device_address+"/feedrate", int64(collector.Feedrate))
	UpdateNodeValueAtAddress(node_ns, device_address+"/feed_override", int64(collector.FeedOverride))
	UpdateNodeValueAtAddress(node_ns, device_address+"/jog_override", float64(collector.JogOverride))
	UpdateNodeValueAtAddress(node_ns, device_address+"/jog_speed", int64(collector.JogSpeed))
	UpdateNodeValueAtAddress(node_ns, device_address+"/current_load", float64(collector.CurrentLoad))
	UpdateNodeValueAtAddress(node_ns, device_address+"/current_load_percent", float64(collector.CurrentLoadPercent))
	UpdateNodeValueAtAddress(node_ns, device_address+"/servo_load_x", int64(GetMapValue(collector.ServoLoads, "X")))
	UpdateNodeValueAtAddress(node_ns, device_address+"/servo_load_z", int64(GetMapValue(collector.ServoLoads, "Z")))
	UpdateNodeValueAtAddress(node_ns, device_address+"/absolute_position_x", int64(GetMapValue(collector.AbsolutePositions, "X")))
	UpdateNodeValueAtAddress(node_ns, device_address+"/absolute_position_z", int64(GetMapValue(collector.AbsolutePositions, "Z")))
	UpdateNodeValueAtAddress(node_ns, device_address+"/machine_position_x", int64(GetMapValue(collector.MachinePositions, "X")))
	UpdateNodeValueAtAddress(node_ns, device_address+"/machine_position_z", int64(GetMapValue(collector.MachinePositions, "Z")))
	UpdateNodeValueAtAddress(node_ns, device_address+"/relative_position_x", int64(GetMapValue(collector.RelativePositions, "X")))
	UpdateNodeValueAtAddress(node_ns, device_address+"/relative_position_z", int64(GetMapValue(collector.RelativePositions, "Z")))
	// Spindle data
	UpdateNodeValueAtAddress(node_ns, device_address+"/spindle_speed", int64(collector.SpindleSpeed))
	UpdateNodeValueAtAddress(node_ns, device_address+"/spindle_param_speed", int64(collector.SpindleParamSpeed))
	UpdateNodeValueAtAddress(node_ns, device_address+"/spindle_override", int16(collector.SpindleOverride))
	UpdateNodeValueAtAddress(node_ns, device_address+"/spindle_motor_speed_s", int64(GetMapValue(collector.SpindleMotorSpeed, "S")))
	UpdateNodeValueAtAddress(node_ns, device_address+"/spindle_load_names", MapKeys(collector.SpindleLoad))
	UpdateNodeValueAtAddress(node_ns, device_address+"/spindle_load", MapValues(collector.SpindleLoad))
	// Alarm data
	UpdateNodeValueAtAddress(node_ns, device_address+"/emergency", int16(collector.Emergency))
	UpdateNodeValueAtAddress(node_ns, device_address+"/alarm", int16(collector.Alarm))
	// Other data
	UpdateNodeValueAtAddress(node_ns, device_address+"/power_on_time", int64(collector.PowerOnTime))
	UpdateNodeValueAtAddress(node_ns, device_address+"/operation_time", int64(collector.OperationTime))
	UpdateNodeValueAtAddress(node_ns, device_address+"/cutting_time", int64(collector.CuttingTime))
	UpdateNodeValueAtAddress(node_ns, device_address+"/cycle_time", int64(collector.CycleTime))
	UpdateNodeValueAtAddress(node_ns, device_address+"/series_number", string(collector.SeriesNumber))
	UpdateNodeValueAtAddress(node_ns, device_address+"/version_number", string(collector.VersionNumber))
	// errors data
	UpdateNodeValueAtAddress(node_ns, device_address+"/errors", []int16(collector.Errors))
	UpdateNodeValueAtAddress(node_ns, device_address+"/errors_str", []string(collector.ErrorsStr))
}

func CreateDeviceNodes(devices []Device, node_ns *server.NodeNameSpace) {
	node_obj := node_ns.Objects()
	device_map = make(map[string]string)
	for index := range devices {
		device_folder := GetFolderNode(node_ns, node_obj, devices[index].Name)
		//device data
		AddVariableNode(node_ns, device_folder, "port", int64(0))
		AddVariableNode(node_ns, device_folder, "name", "")
		AddVariableNode(node_ns, device_folder, "address", "")
		AddVariableNode(node_ns, device_folder, "series", "")
		//mode data
		AddVariableNode(node_ns, device_folder, "aut", int16(0))
		AddVariableNode(node_ns, device_folder, "run", int16(0))
		AddVariableNode(node_ns, device_folder, "edit", int16(0))
		AddVariableNode(node_ns, device_folder, "shutdowns", int16(0))
		AddVariableNode(node_ns, device_folder, "hight_speed", int16(0))
		AddVariableNode(node_ns, device_folder, "motion", int16(0))
		AddVariableNode(node_ns, device_folder, "mstb", int16(0))
		AddVariableNode(node_ns, device_folder, "load_excess", int64(0))
		//program data
		AddVariableNode(node_ns, device_folder, "main_prog_number", int16(0))
		AddVariableNode(node_ns, device_folder, "sub_prog_number", int16(0))
		AddVariableNode(node_ns, device_folder, "parts_count", int64(0))
		AddVariableNode(node_ns, device_folder, "tool_number", int64(0))
		AddVariableNode(node_ns, device_folder, "frame_number", int64(0))
		AddVariableNode(node_ns, device_folder, "frame", "")
		//axes data
		AddVariableNode(node_ns, device_folder, "feedrate", int64(0))
		AddVariableNode(node_ns, device_folder, "feed_override", int64(0))
		AddVariableNode(node_ns, device_folder, "jog_override", float64(0))
		AddVariableNode(node_ns, device_folder, "jog_speed", int64(0))
		AddVariableNode(node_ns, device_folder, "current_load", float64(0))
		AddVariableNode(node_ns, device_folder, "current_load_percent", float64(0))
		AddVariableNode(node_ns, device_folder, "servo_load_x", int64(0))
		AddVariableNode(node_ns, device_folder, "servo_load_z", int64(0))
		AddVariableNode(node_ns, device_folder, "absolute_position_x", int64(0))
		AddVariableNode(node_ns, device_folder, "absolute_position_z", int64(0))
		AddVariableNode(node_ns, device_folder, "machine_position_x", int64(0))
		AddVariableNode(node_ns, device_folder, "machine_position_z", int64(0))
		AddVariableNode(node_ns, device_folder, "relative_position_x", int64(0))
		AddVariableNode(node_ns, device_folder, "relative_position_z", int64(0))
		//spindle data
		AddVariableNode(node_ns, device_folder, "spindle_speed", int64(0))
		AddVariableNode(node_ns, device_folder, "spindle_param_speed", int64(0))
		AddVariableNode(node_ns, device_folder, "spindle_override", int64(0))
		AddVariableNode(node_ns, device_folder, "spindle_motor_speed_s", int64(0))
		SetValueRank(AddVariableNode(node_ns, device_folder, "spindle_load", []int64{}), 1)
		SetValueRank(AddVariableNode(node_ns, device_folder, "spindle_load_names", []string{}), 1)
		//alarm data
		AddVariableNode(node_ns, device_folder, "emergency", int16(0))
		AddVariableNode(node_ns, device_folder, "alarm", int16(0))
		//other data
		AddVariableNode(node_ns, device_folder, "power_on_time", int64(0))
		AddVariableNode(node_ns, device_folder, "operation_time", int64(0))
		AddVariableNode(node_ns, device_folder, "cutting_time", int64(0))
		AddVariableNode(node_ns, device_folder, "cycle_time", int64(0))
		AddVariableNode(node_ns, device_folder, "series_number", "")
		AddVariableNode(node_ns, device_folder, "version_number", "")
		//errors data
		SetValueRank(AddVariableNode(node_ns, device_folder, "errors", make([]int16, 28)), 1)
		SetValueRank(AddVariableNode(node_ns, device_folder, "errors_str", make([]string, 28)), 1)
		device_map[devices[index].Name] = device_folder.ID().String()
	}
}

func inicialize() {
	flag.BoolVar(&debug.Enable, "debug", config.Server.Debug, "enable debug logging")
	flag.Parse()
	log.SetFlags(0)

	var opts []server.Option

	security_options := GetPoliciesOptions(config.Server.Security, available_policies, available_sec_modes)
	if security_options == nil {
		opts = append(opts, server.EnableSecurity("None", ua.MessageSecurityModeNone))
	} else {
		opts = append(opts, security_options...)
	}

	auth_options := GetAuthModeOptions(config.Server.AuthModes, available_auth_modes)
	if auth_options == nil {
		opts = append(opts, server.EnableAuthMode(ua.UserTokenTypeAnonymous))
	} else {
		opts = append(opts, auth_options...)
	}

	endpoints := config.Server.Endpoints
	endpoint_options := GetEndpointOptions(endpoints)
	if endpoint_options == nil {
		port := 4840
		endpoints = []ImportEndpoint{{Endpoint: "localhost", Port: port}}
		hostname, err := os.Hostname()
		if err == nil {
			endpoints = append(endpoints, ImportEndpoint{Endpoint: hostname, Port: port})
		}
		endpoint_options = GetEndpointOptions(endpoints)
	}
	opts = append(opts, endpoint_options...)

	_logger := Logger(1)
	opts = append(opts,
		server.SetLogger(_logger),
	)

	make_cert := config.Server.MakeCert
	cert_pem_path := filepath.Join(plugin_dir, *certfile)
	cert_der_path := filepath.Join(plugin_dir, "cert.der")
	key_pem_path := filepath.Join(plugin_dir, *keyfile)
	if make_cert {
		var endpoints_str []string
		if endpoints == nil {
		}
		for _, imp_endpoint := range endpoints {
			endpoints_str = append(endpoints_str, imp_endpoint.Endpoint)
		}

		cert_created := false
		if _, err := os.Stat(cert_pem_path); err == nil {
			logger.Printf("Файл %s уже существует, пропускаем генерацию", *certfile)
			cert_created = true
		}
		key_created := false
		if _, err := os.Stat(key_pem_path); err == nil {
			logger.Printf("Файл %s уже существует, пропускаем генерацию", *keyfile)
			key_created = true
		}
		cert_der_created := false
		if _, err := os.Stat(cert_der_path); err == nil {
			logger.Printf("Файл %s уже существует, пропускаем генерацию", "cert.der")
			cert_der_created = true
		}

		if !cert_created && !key_created && !cert_der_created {
			c, k, err := GenerateCert(endpoints_str, 2048, time.Minute*60*24*365*10)
			if err != nil {
				logger.Fatalf("problem creating cert: %v", err)
			}
			err = os.WriteFile(cert_pem_path, c, 0644)
			if err != nil {
				logger.Fatalf("problem writing cert: %v", err)
			}
			err = os.WriteFile(key_pem_path, k, 0644)
			if err != nil {
				logger.Fatalf("problem writing key: %v", err)
			}
			der, _ := pem.Decode(c)
			if der == nil {
				logger.Fatalf("failed to parse PEM block for cert")
			}
			err = os.WriteFile(cert_der_path, der.Bytes, 0644)
			if err != nil {
				logger.Fatalf("problem writing DER cert: %v", err)
			}
		}
	}

	if StrContais("Certificate", config.Server.AuthModes) {
		var cert []byte
		logger.Printf("Loading cert/key from %s/%s", *certfile, *keyfile)
		c, err := tls.LoadX509KeyPair(cert_pem_path, key_pem_path)
		if err != nil {
			logger.Printf("Failed to load certificate: %s", err)
		} else {
			pk, ok := c.PrivateKey.(*rsa.PrivateKey)
			if !ok {
				logger.Fatalf("Invalid private key")
			}
			cert = c.Certificate[0]
			opts = append(opts, server.PrivateKey(pk), server.Certificate(cert))
		}

		lcerts := config.Server.TrustedCerts
		if len(lcerts) > 0 {
			for _, lcert := range lcerts {
				opt := AddCert(lcert)
				if opt != nil {
					opts = append(opts, opt)
					fmt.Println(lcert)
				}
			}
		}

		lkeys := config.Server.TrustedKeys
		if len(lkeys) > 0 {
			for _, lkey := range lkeys {
				opt := AddPK(lkey)
				if opt != nil {
					opts = append(opts, opt)
					fmt.Println(lkey)
				}
			}
		}
	}

	_server = server.New(opts...)
	root_ns, _ := _server.Namespace(0)
	root_obj_node := root_ns.Objects()
	node_ns := server.NewNodeNameSpace(_server, "Fanuc Devices")
	nns_obj := node_ns.Objects()
	nns_obj.SetDescription("Fanuc devices data", "Fanuc devices data")
	root_obj_node.AddRef(nns_obj, id.HasComponent, true)
	CreateDeviceNodes(config.Devices, node_ns)
	if config.Server.MakeCSV {
		for key := range device_map {
			MakeCSV(GetTagsAtOpcNodes(key), key, plugin_dir)
		}
	}
}

func start() {
	if err := _server.Start(context.Background()); err != nil {
		log.Fatalf("Ошибка запуска сервера: %s", err)
	}
	defer _server.Close()
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	defer signal.Stop(sigch)
	<-sigch
}
