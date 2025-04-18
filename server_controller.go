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
var device_addresses []string

type Logger int

func (l Logger) Debug(msg string, args ...any) {
	if l < 0 {
		log.Printf(msg, args...)
	}
}
func (l Logger) Info(msg string, args ...any) {
	if l < 1 {
		log.Printf(msg, args...)
	}
}
func (l Logger) Warn(msg string, args ...any) {
	if l < 2 {
		log.Printf(msg, args...)
	}
}
func (l Logger) Error(msg string, args ...any) {
	if l < 3 {
		log.Printf(msg, args...)
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

func GetDeviceNodes(device_name string) []*server.Node {
	var result []*server.Node
	addresses := []string{
		"/device_data/name",
		"/device_data/address",
		"/device_data/port",
		"/device_data/series",
		"/mode_data/mode",
		"/mode_data/run_state",
		"/mode_data/status",
		"/mode_data/shutdowns",
		"/mode_data/hight_speed",
		"/mode_data/axis_motion",
		"/mode_data/mstb",
		"/mode_data/load_excess",
		"/mode_data/mode_str",
		"/mode_data/run_state_str",
		"/mode_data/status_str",
		"/mode_data/shutdowns_str",
		"/mode_data/hight_speed_str",
		"/mode_data/axis_motion_str",
		"/mode_data/mstb_str",
		"/mode_data/load_excess_str",
		"/mode_data/mode_errors",
		"/mode_data/mode_errors_str",
		"/program_data/frame",
		"/program_data/main_prog_number",
		"/program_data/sub_prog_number",
		"/program_data/parts_count",
		"/program_data/tool_number",
		"/program_data/frame_number",
		"/program_data/prg_errors",
		"/program_data/prg_errors_str",
		"/axes_data/feedrate",
		"/axes_data/feed_override",
		"/axes_data/jog_override",
		"/axes_data/jog_speed",
		"/axes_data/current_load",
		"/axes_data/current_load_percent",
		"/axes_data/servo_loads",
		"/axes_data/axes_errors",
		"/axes_data/axes_errors_str",
		"/spindle_data/spindle_speed",
		"/spindle_data/spindle_param_speed",
		"/spindle_data/spindle_override",
		"/spindle_data/spindle_motor_speed",
		"/spindle_data/spindle_load",
		"/spindle_data/spindle_errors",
		"/spindle_data/spindle_errors_str",
		"/alarm_data/emergency",
		"/alarm_data/alarm_status",
		"/alarm_data/emergency_str",
		"/alarm_data/alarm_status_str",
		"/alarm_data/alarm_errors",
		"/alarm_data/alarm_errors_str",
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

func UpdateDeviceNodes(collectors []CollectorData) {
	node_ns := GetNodeNamespace(_server, fanuc_ns)
	if node_ns != nil {
		var device_address string
		if len(device_addresses) == len(collectors) {
			for index, collector := range collectors {
				device_address = device_addresses[index]
				// Device data
				UpdateNodeValueAtAddress(node_ns, device_address+"/device_data/name", string(collector.Device.Name))
				UpdateNodeValueAtAddress(node_ns, device_address+"/device_data/address", string(collector.Device.Address))
				UpdateNodeValueAtAddress(node_ns, device_address+"/device_data/port", int64(collector.Device.Port))
				UpdateNodeValueAtAddress(node_ns, device_address+"/device_data/series", string(collector.Device.Series))
				// Mode data
				UpdateNodeValueAtAddress(node_ns, device_address+"/mode_data/mode", int16(collector.Mode.Mode))
				UpdateNodeValueAtAddress(node_ns, device_address+"/mode_data/run_state", int16(collector.Mode.RunState))
				UpdateNodeValueAtAddress(node_ns, device_address+"/mode_data/status", int16(collector.Mode.Status))
				UpdateNodeValueAtAddress(node_ns, device_address+"/mode_data/shutdowns", int16(collector.Mode.Shutdowns))
				UpdateNodeValueAtAddress(node_ns, device_address+"/mode_data/hight_speed", int16(collector.Mode.HightSpeed))
				UpdateNodeValueAtAddress(node_ns, device_address+"/mode_data/axis_motion", int16(collector.Mode.AxisMotion))
				UpdateNodeValueAtAddress(node_ns, device_address+"/mode_data/mstb", int16(collector.Mode.Mstb))
				UpdateNodeValueAtAddress(node_ns, device_address+"/mode_data/load_excess", int64(collector.Mode.LoadExcess))
				UpdateNodeValueAtAddress(node_ns, device_address+"/mode_data/mode_str", string(collector.Mode.ModeStr))
				UpdateNodeValueAtAddress(node_ns, device_address+"/mode_data/run_state_str", string(collector.Mode.RunStateStr))
				UpdateNodeValueAtAddress(node_ns, device_address+"/mode_data/status_str", string(collector.Mode.StatusStr))
				UpdateNodeValueAtAddress(node_ns, device_address+"/mode_data/shutdowns_str", string(collector.Mode.ShutdownsStr))
				UpdateNodeValueAtAddress(node_ns, device_address+"/mode_data/hight_speed_str", string(collector.Mode.HightSpeedStr))
				UpdateNodeValueAtAddress(node_ns, device_address+"/mode_data/axis_motion_str", string(collector.Mode.AxisMotionStr))
				UpdateNodeValueAtAddress(node_ns, device_address+"/mode_data/mstb_str", string(collector.Mode.MstbStr))
				UpdateNodeValueAtAddress(node_ns, device_address+"/mode_data/load_excess_str", string(collector.Mode.LoadExcessStr))
				UpdateNodeValueAtAddress(node_ns, device_address+"/mode_data/mode_errors", []int16(collector.Mode.ModeErrors))
				UpdateNodeValueAtAddress(node_ns, device_address+"/mode_data/mode_errors_str", []string(collector.Mode.ModeErrorsStr))
				// Program data
				UpdateNodeValueAtAddress(node_ns, device_address+"/program_data/frame", string(collector.Program.Frame))
				UpdateNodeValueAtAddress(node_ns, device_address+"/program_data/main_prog_number", int64(collector.Program.MainProgNumber))
				UpdateNodeValueAtAddress(node_ns, device_address+"/program_data/sub_prog_number", int64(collector.Program.SubProgNumber))
				UpdateNodeValueAtAddress(node_ns, device_address+"/program_data/parts_count", int64(collector.Program.PartsCount))
				UpdateNodeValueAtAddress(node_ns, device_address+"/program_data/tool_number", int64(collector.Program.ToolNumber))
				UpdateNodeValueAtAddress(node_ns, device_address+"/program_data/frame_number", int64(collector.Program.FrameNumber))
				UpdateNodeValueAtAddress(node_ns, device_address+"/program_data/prg_errors", []int16(collector.Program.ProgErrors))
				UpdateNodeValueAtAddress(node_ns, device_address+"/program_data/prg_errors_str", []string(collector.Program.ProgErrorsStr))
				// Axes data
				UpdateNodeValueAtAddress(node_ns, device_address+"/axes_data/feedrate", int64(collector.Axes.FeedRate))
				UpdateNodeValueAtAddress(node_ns, device_address+"/axes_data/feed_override", int64(collector.Axes.FeedOverride))
				UpdateNodeValueAtAddress(node_ns, device_address+"/axes_data/jog_override", float64(collector.Axes.JogOverride))
				UpdateNodeValueAtAddress(node_ns, device_address+"/axes_data/jog_speed", int64(collector.Axes.JogSpeed))
				UpdateNodeValueAtAddress(node_ns, device_address+"/axes_data/current_load", float64(collector.Axes.CurrentLoad))
				UpdateNodeValueAtAddress(node_ns, device_address+"/axes_data/current_load_percent", float64(collector.Axes.CurrentLoadPercent))
				UpdateNodeValueAtAddress(node_ns, device_address+"/axes_data/servo_loads", string(MapToStr(collector.Axes.ServoLoads)))
				UpdateNodeValueAtAddress(node_ns, device_address+"/axes_data/axes_errors", []int16(collector.Axes.AxesErrors))
				UpdateNodeValueAtAddress(node_ns, device_address+"/axes_data/axes_errors_str", []string(collector.Axes.AxesErrorsStr))
				// Spindle data
				UpdateNodeValueAtAddress(node_ns, device_address+"/spindle_data/spindle_speed", int64(collector.Spindle.SpindleSpeed))
				UpdateNodeValueAtAddress(node_ns, device_address+"/spindle_data/spindle_param_speed", int64(collector.Spindle.SpindleSpeedParam))
				UpdateNodeValueAtAddress(node_ns, device_address+"/spindle_data/spindle_override", int64(collector.Spindle.SpindleOverride))
				UpdateNodeValueAtAddress(node_ns, device_address+"/spindle_data/spindle_motor_speed", string(MapToStr(collector.Spindle.SpindleMotorSpeed)))
				UpdateNodeValueAtAddress(node_ns, device_address+"/spindle_data/spindle_load", string(MapToStr(collector.Spindle.SpindleLoad)))
				UpdateNodeValueAtAddress(node_ns, device_address+"/spindle_data/spindle_errors", []int16(collector.Spindle.SpindleErrors))
				UpdateNodeValueAtAddress(node_ns, device_address+"/spindle_data/spindle_errors_str", []string(collector.Spindle.SpindleErrorsStr))
				// Alarm data
				UpdateNodeValueAtAddress(node_ns, device_address+"/alarm_data/emergency", int16(collector.Alarm.Emergency))
				UpdateNodeValueAtAddress(node_ns, device_address+"/alarm_data/alarm_status", int16(collector.Alarm.AlarmStatus))

				UpdateNodeValueAtAddress(node_ns, device_address+"/alarm_data/emergency_str", string(collector.Alarm.EmergencyStr))
				UpdateNodeValueAtAddress(node_ns, device_address+"/alarm_data/alarm_status_str", string(collector.Alarm.AlarmStatusStr))
				UpdateNodeValueAtAddress(node_ns, device_address+"/alarm_data/alarm_errors", []int16(collector.Alarm.AlarmErrors))
				UpdateNodeValueAtAddress(node_ns, device_address+"/alarm_data/alarm_errors_str", []string(collector.Alarm.AlarmErrorsStr))
			}
		}
	}
}

func CreateCollectorNodes(collectors []CollectorData, node_ns *server.NodeNameSpace) {
	node_obj := node_ns.Objects()
	for index := range collectors {
		device_folder := GetFolderNode(node_ns, node_obj, collectors[index].Device.Name)
		//Device data
		device_data_folder := GetFolderNode(node_ns, device_folder, "device_data")
		AddVariableNode(node_ns, device_data_folder, "port", int64(0))
		AddVariableNode(node_ns, device_data_folder, "name", "")
		AddVariableNode(node_ns, device_data_folder, "address", "")
		AddVariableNode(node_ns, device_data_folder, "series", "")
		//Mode data
		mode_folder := GetFolderNode(node_ns, device_folder, "mode_data")
		AddVariableNode(node_ns, mode_folder, "mode", int16(0))
		AddVariableNode(node_ns, mode_folder, "run_state", int16(0))
		AddVariableNode(node_ns, mode_folder, "status", int16(0))
		AddVariableNode(node_ns, mode_folder, "shutdowns", int16(0))
		AddVariableNode(node_ns, mode_folder, "hight_speed", int16(0))
		AddVariableNode(node_ns, mode_folder, "axis_motion", int16(0))
		AddVariableNode(node_ns, mode_folder, "mstb", int16(0))
		AddVariableNode(node_ns, mode_folder, "load_excess", int64(0))
		AddVariableNode(node_ns, mode_folder, "mode_str", "")
		AddVariableNode(node_ns, mode_folder, "run_state_str", "")
		AddVariableNode(node_ns, mode_folder, "status_str", "")
		AddVariableNode(node_ns, mode_folder, "shutdowns_str", "")
		AddVariableNode(node_ns, mode_folder, "hight_speed_str", "")
		AddVariableNode(node_ns, mode_folder, "axis_motion_str", "")
		AddVariableNode(node_ns, mode_folder, "mstb_str", "")
		AddVariableNode(node_ns, mode_folder, "load_excess_str", "")
		AddVariableNode(node_ns, mode_folder, "mode_errors", make([]int16, 8))
		AddVariableNode(node_ns, mode_folder, "mode_errors_str", make([]string, 8))
		//Program data
		program_folder := GetFolderNode(node_ns, device_folder, "program_data")
		AddVariableNode(node_ns, program_folder, "main_prog_number", int64(0))
		AddVariableNode(node_ns, program_folder, "sub_prog_number", int64(0))
		AddVariableNode(node_ns, program_folder, "parts_count", int64(0))
		AddVariableNode(node_ns, program_folder, "tool_number", int64(0))
		AddVariableNode(node_ns, program_folder, "frame_number", int64(0))
		AddVariableNode(node_ns, program_folder, "frame", "")
		AddVariableNode(node_ns, program_folder, "program_errors", make([]int16, 6))
		AddVariableNode(node_ns, program_folder, "program_errors_str", make([]string, 6))
		//Axes data
		axes_folder := GetFolderNode(node_ns, device_folder, "axes_data")
		AddVariableNode(node_ns, axes_folder, "feedrate", int64(0))
		AddVariableNode(node_ns, axes_folder, "feed_override", int64(0))
		AddVariableNode(node_ns, axes_folder, "jog_override", float64(0))
		AddVariableNode(node_ns, axes_folder, "jog_speed", int64(0))
		AddVariableNode(node_ns, axes_folder, "current_load", float64(0))
		AddVariableNode(node_ns, axes_folder, "current_load_percent", float64(0))
		AddVariableNode(node_ns, axes_folder, "servo_loads", "")
		AddVariableNode(node_ns, axes_folder, "axes_errors", make([]int16, 7))
		AddVariableNode(node_ns, axes_folder, "axes_errors_str", make([]string, 7))
		//Spindle data
		spindle_folder := GetFolderNode(node_ns, device_folder, "spindle_data")
		AddVariableNode(node_ns, spindle_folder, "spindle_speed", int64(0))
		AddVariableNode(node_ns, spindle_folder, "spindle_param_speed", int64(0))
		AddVariableNode(node_ns, spindle_folder, "spindle_override", int64(0))
		AddVariableNode(node_ns, spindle_folder, "spindle_motor_speed", "")
		AddVariableNode(node_ns, spindle_folder, "spindle_load", "")
		AddVariableNode(node_ns, spindle_folder, "spindle_errors", make([]int16, 5))
		AddVariableNode(node_ns, spindle_folder, "spindle_errors_str", make([]string, 5))
		// Alarm data
		alarm_folder := GetFolderNode(node_ns, device_folder, "alarm_data")
		AddVariableNode(node_ns, alarm_folder, "emergency", "")
		AddVariableNode(node_ns, alarm_folder, "alarm_status", "")
		AddVariableNode(node_ns, alarm_folder, "emergency_str", "")
		AddVariableNode(node_ns, alarm_folder, "alarm_status_str", "")
		AddVariableNode(node_ns, alarm_folder, "alarm_errors", make([]int16, 2))
		AddVariableNode(node_ns, alarm_folder, "alarm_errors_str", make([]string, 2))
		device_addresses = append(device_addresses, device_folder.ID().String())
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

	logger := Logger(1)
	opts = append(opts,
		server.SetLogger(logger),
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
			log.Printf("Файл %s уже существует, пропускаем генерацию", *certfile)
			cert_created = true
		}
		key_created := false
		if _, err := os.Stat(key_pem_path); err == nil {
			log.Printf("Файл %s уже существует, пропускаем генерацию", *keyfile)
			key_created = true
		}
		cert_der_created := false
		if _, err := os.Stat(cert_der_path); err == nil {
			log.Printf("Файл %s уже существует, пропускаем генерацию", "cert.der")
			cert_der_created = true
		}

		if !cert_created && !key_created && !cert_der_created {
			c, k, err := GenerateCert(endpoints_str, 2048, time.Minute*60*24*365*10)
			if err != nil {
				log.Fatalf("problem creating cert: %v", err)
			}
			err = os.WriteFile(cert_pem_path, c, 0644)
			if err != nil {
				log.Fatalf("problem writing cert: %v", err)
			}
			err = os.WriteFile(key_pem_path, k, 0644)
			if err != nil {
				log.Fatalf("problem writing key: %v", err)
			}
			der, _ := pem.Decode(c)
			if der == nil {
				log.Fatalf("failed to parse PEM block for cert")
			}
			err = os.WriteFile(cert_der_path, der.Bytes, 0644)
			if err != nil {
				log.Fatalf("problem writing DER cert: %v", err)
			}
		}
	}

	if StrContais("Certificate", config.Server.AuthModes) {
		var cert []byte
		log.Printf("Loading cert/key from %s/%s", *certfile, *keyfile)
		c, err := tls.LoadX509KeyPair(cert_pem_path, key_pem_path)
		if err != nil {
			log.Printf("Failed to load certificate: %s", err)
		} else {
			pk, ok := c.PrivateKey.(*rsa.PrivateKey)
			if !ok {
				log.Fatalf("Invalid private key")
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
	CreateCollectorNodes(collectors_data.Collectors, node_ns)

	if config.Server.MakeCSV {
		for index := range device_addresses {
			device_name := config.Devices[index].Name
			MakeCSV(GetTagsAtOpcNodes(device_name), device_name, plugin_dir)
		}
	}
}

func start() {
	if err := _server.Start(context.Background()); err != nil {
		log.Fatalf("Error starting server, exiting: %s", err)
	}
	defer _server.Close()
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	defer signal.Stop(sigch)
	<-sigch
}
