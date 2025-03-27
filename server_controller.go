// Copyright 2018-2020 opcua authors. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gopcua/opcua/debug"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/server"
	"github.com/gopcua/opcua/server/attrs"
	"github.com/gopcua/opcua/ua"
)

var (
	endpoint = flag.String("endpoint", "0.0.0.0", "OPC UA Endpoint URL")
	port     = flag.Int("port", 4840, "OPC UA Endpoint port")
	certfile = flag.String("cert", "cert.pem", "Path to certificate file")
	keyfile  = flag.String("key", "key.pem", "Path to PEM Private Key file")
	gencert  = flag.Bool("gen-cert", false, "Generate a new certificate")
)

var available_policies []string = []string{
	"None",
	"Basic128Rsa15",
	"Basic256",
	"Basic256Sha256",
	"Aes128_Sha256_RsaOaep",
	"Aes128_Sha256_RsaOaep",
}

var available_sec_modes []string = []string{
	"None",
	"Sign",
	"SignAndEncrypt",
}

var available_auth_modes []string = []string{
	"Anonymous",
	"Username",
	"Certificate",
}

var loaded_policies []string
var loaded_auth_modes []string

var _server *server.Server
var _node_ns *server.NodeNameSpace

type ModeNodes struct {
	Mode       *server.Node
	RunState   *server.Node
	Status     *server.Node
	Shutdowns  *server.Node
	HightSpeed *server.Node
	AxisMotion *server.Node
	Mstb       *server.Node
	LoadExcess *server.Node
	ModeErr    *server.Node
}

type ProgramNodes struct {
	Frame          *server.Node
	MainProgNumber *server.Node
	SubProgNumber  *server.Node
	PartsCount     *server.Node
	ToolNumber     *server.Node
	FrameNumber    *server.Node
	PrgErr         *server.Node
}

type AxisNodes struct {
	FeedRate           *server.Node
	FeedOverride       *server.Node
	JogOverride        *server.Node
	JogSpeed           *server.Node
	CurrentLoad        *server.Node
	CurrentLoadPercent *server.Node
	ServoLoads         *server.Node
	AxesErr            *server.Node
}

type SpindleNodes struct {
	SpindleSpeed      *server.Node
	SpindleSpeedParam *server.Node
	SpindleMotorSpeed *server.Node
	SpindleLoad       *server.Node
	SpindleOverride   *server.Node
	SpindleErr        *server.Node
}

type DeviceNodes struct {
	name    *server.Node
	address *server.Node
	port    *server.Node
	series  *server.Node
}

type CollectorNodes struct {
	device_nodes  DeviceNodes
	mode_nodes    ModeNodes
	prog_nodes    ProgramNodes
	axis_nodes    AxisNodes
	spindle_nodes SpindleNodes
}

var col_nodes []CollectorNodes

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

func StrContais(str string, slice []string) bool {
	for _, value := range slice {
		if value == str {
			return true
		}
	}
	return false
}

func GetSecurityMode(policy string, mode string) ua.MessageSecurityMode {
	if policy == "None" {
		return ua.MessageSecurityModeNone
	}
	switch mode {
	case "Sign":
		return ua.MessageSecurityModeSign
	case "SignAndEncrypt":
		return ua.MessageSecurityModeSignAndEncrypt
	default:
		return ua.MessageSecurityModeSign
	}
}

func GetPoliciesOptions(policies map[string]string) []server.Option {
	if policies == nil {
		fmt.Println("Отсутсвуют политики безопасности")
		return nil
	}
	if len(policies) == 0 {
		fmt.Println("Список политик безопасности пуст")
		return nil
	}
	options := []server.Option{}
	for policy, mode := range policies {
		policy_access := StrContais(policy, available_policies)
		mode_access := StrContais(mode, available_sec_modes)
		if policy_access && mode_access {
			merge := policy + mode
			if !StrContais(merge, loaded_policies) {
				fmt.Println(merge)
				options = append(options, server.EnableSecurity(policy, GetSecurityMode(policy, mode)))
				loaded_policies = append(loaded_policies, merge)
			}
		}
	}
	if len(options) == 0 {
		return nil
	}
	return options
}

func GetAuthMode(mode string) ua.UserTokenType {
	switch mode {
	case "Anonymous":
		return ua.UserTokenTypeAnonymous
	case "Username":
		return ua.UserTokenTypeUserName
	case "Certificate":
		return ua.UserTokenTypeCertificate
	default:
		return ua.UserTokenTypeAnonymous
	}
}

func GetAuthModeOptions(auth_modes []string) []server.Option {
	if auth_modes == nil {
		return nil
	}
	if len(auth_modes) == 0 {
		return nil
	}
	options := []server.Option{}
	for _, mode := range auth_modes {
		auth_access := StrContais(mode, available_auth_modes)
		if auth_access {
			if !StrContais(mode, loaded_auth_modes) {
				options = append(options, server.EnableAuthMode(GetAuthMode(mode)))
				loaded_auth_modes = append(loaded_auth_modes, mode)
			}
		}
	}
	if len(options) == 0 {
		return nil
	}
	return options
}

func inicialize() {
	flag.BoolVar(&debug.Enable, "debug", false, "enable debug logging")
	flag.Parse()
	log.SetFlags(0)

	var opts []server.Option

	security_options := GetPoliciesOptions(config.Server.Security)
	if security_options == nil {
		opts = append(opts, server.EnableSecurity("None", ua.MessageSecurityModeNone))
	} else {
		opts = append(opts, security_options...)
	}

	auth_options := GetAuthModeOptions(config.Server.AuthModes)
	if auth_options == nil {
		opts = append(opts, server.EnableAuthMode(ua.UserTokenTypeAnonymous))
	} else {
		opts = append(opts, auth_options...)
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Error getting host name %v", err)
	}

	opts = append(opts,
		server.EndPoint(*endpoint, *port),
		server.EndPoint("localhost", *port),
		server.EndPoint(hostname, *port),
	)

	logger := Logger(1)
	opts = append(opts,
		server.SetLogger(logger),
	)

	if *gencert {
		endpoints := []string{
			"localhost",
			hostname,
			*endpoint,
		}

		c, k, err := GenerateCert(endpoints, 4096, time.Minute*60*24*365*10)
		if err != nil {
			log.Fatalf("problem creating cert: %v", err)
		}
		err = os.WriteFile(*certfile, c, 0)
		if err != nil {
			log.Fatalf("problem writing cert: %v", err)
		}
		err = os.WriteFile(*keyfile, k, 0)
		if err != nil {
			log.Fatalf("problem writing key: %v", err)
		}

	}

	var cert []byte
	if *gencert || (*certfile != "" && *keyfile != "") {
		log.Printf("Loading cert/key from %s/%s", *certfile, *keyfile)
		c, err := tls.LoadX509KeyPair(*certfile, *keyfile)
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
	}

	_server = server.New(opts...)
	root_ns, _ := _server.Namespace(0)
	root_obj_node := root_ns.Objects()
	nodeNS := server.NewNodeNameSpace(_server, "Fanuc Devices")
	nodeNS.Objects().SetDescription("Fanuc devices data", "Fanuc devices data")
	_node_ns = nodeNS
	nns_obj := nodeNS.Objects()
	root_obj_node.AddRef(nns_obj, id.HasComponent, true)
	CreateCollectorNodes(collectors_data, nodeNS)
}

func AddVariableNode(node_ns *server.NodeNameSpace, node *server.Node, name string, value any) *server.Node {
	parent_id := node.ID().StringID()
	if parent_id != "" {
		parent_id += "/"
	}
	node_id := ua.NewStringExpandedNodeID(node_ns.ID(), parent_id+name)
	attributes := map[ua.AttributeID]*ua.DataValue{
		ua.AttributeIDNodeID:          server.DataValueFromValue(node_id),
		ua.AttributeIDNodeClass:       server.DataValueFromValue(uint32(ua.NodeClassVariable)),
		ua.AttributeIDBrowseName:      server.DataValueFromValue(attrs.BrowseName(name)),
		ua.AttributeIDDisplayName:     server.DataValueFromValue(attrs.DisplayName(name, name)),
		ua.AttributeIDDescription:     server.DataValueFromValue(&ua.LocalizedText{Locale: name, Text: name}),
		ua.AttributeIDValue:           server.DataValueFromValue(value),
		ua.AttributeIDDataType:        server.DataValueFromValue(node_id),
		ua.AttributeIDWriteMask:       server.DataValueFromValue(uint32(0)),
		ua.AttributeIDUserWriteMask:   server.DataValueFromValue(uint32(0)),
		ua.AttributeIDAccessLevel:     server.DataValueFromValue(uint8(0x05)),
		ua.AttributeIDUserAccessLevel: server.DataValueFromValue(uint8(0x05)),
		ua.AttributeIDHistorizing:     server.DataValueFromValue(bool(false)),
		ua.AttributeIDValueRank:       server.DataValueFromValue(int32(-1)),
	}
	variable := server.NewNode(ua.NewNodeIDFromExpandedNodeID(node_id), attributes, nil, nil)
	variable.SetAttribute(ua.AttributeIDValue, server.DataValueFromValue(value))
	node_ns.AddNode(variable)
	node.AddRef(variable, id.HasComponent, true)
	return variable
}

func GetFolderNode(node_ns *server.NodeNameSpace, node *server.Node, name string) *server.Node {
	parent_id := node.ID().StringID()
	if parent_id != "" {
		parent_id += "/"
	}
	folder_id := ua.NewStringNodeID(node_ns.ID(), parent_id+name)
	attributes := map[ua.AttributeID]*ua.DataValue{
		ua.AttributeIDNodeClass:   server.DataValueFromValue(uint32(ua.NodeClassObject)),
		ua.AttributeIDBrowseName:  server.DataValueFromValue(attrs.BrowseName(name)),
		ua.AttributeIDDescription: server.DataValueFromValue(&ua.LocalizedText{Locale: name, Text: name}),
	}
	folder := server.NewNode(folder_id, attributes, nil, nil)
	node_ns.AddNode(folder)
	node.AddRef(folder, id.HasComponent, true)
	return folder
}

func UpdateValue(node *server.Node, value any) {
	val := ua.DataValue{
		Value:           ua.MustVariant(value),
		SourceTimestamp: time.Now(),
		EncodingMask:    ua.DataValueValue | ua.DataValueSourceTimestamp,
	}
	node.SetAttribute(ua.AttributeIDValue, &val)
	_node_ns.ChangeNotification(node.ID())
}

func UpdateDeviceNodes(col_data *CollectorsData) {
	for index, value := range col_data.Collectors {
		//device data
		UpdateValue(col_nodes[index].device_nodes.name, string(value.Device.Name))
		UpdateValue(col_nodes[index].device_nodes.address, string(value.Device.Address))
		UpdateValue(col_nodes[index].device_nodes.port, int64(value.Device.Port))
		UpdateValue(col_nodes[index].device_nodes.series, string(value.Device.Series))
		//mode data
		UpdateValue(col_nodes[index].mode_nodes.Mode, string(value.Mode.Mode))
		UpdateValue(col_nodes[index].mode_nodes.RunState, string(value.Mode.RunState))
		UpdateValue(col_nodes[index].mode_nodes.Status, string(value.Mode.Status))
		UpdateValue(col_nodes[index].mode_nodes.Shutdowns, string(value.Mode.Shutdowns))
		UpdateValue(col_nodes[index].mode_nodes.HightSpeed, string(value.Mode.HightSpeed))
		UpdateValue(col_nodes[index].mode_nodes.AxisMotion, string(value.Mode.AxisMotion))
		UpdateValue(col_nodes[index].mode_nodes.Mstb, string(value.Mode.LoadExcess))
		UpdateValue(col_nodes[index].mode_nodes.LoadExcess, string(value.Mode.LoadExcess))
		UpdateValue(col_nodes[index].mode_nodes.ModeErr, string(value.Mode.ModeErr))
		//program data
		UpdateValue(col_nodes[index].prog_nodes.Frame, string(value.Program.Frame))
		UpdateValue(col_nodes[index].prog_nodes.MainProgNumber, int64(value.Program.MainProgNumber))
		UpdateValue(col_nodes[index].prog_nodes.SubProgNumber, int64(value.Program.SubProgNumber))
		UpdateValue(col_nodes[index].prog_nodes.PartsCount, int64(value.Program.PartsCount))
		UpdateValue(col_nodes[index].prog_nodes.ToolNumber, int64(value.Program.ToolNumber))
		UpdateValue(col_nodes[index].prog_nodes.FrameNumber, int64(value.Program.FrameNumber))
		UpdateValue(col_nodes[index].prog_nodes.PrgErr, string(value.Program.PrgErr))
		//axis data
		UpdateValue(col_nodes[index].axis_nodes.FeedRate, int64(value.Axes.FeedRate))
		UpdateValue(col_nodes[index].axis_nodes.FeedOverride, int64(value.Axes.FeedOverride))
		UpdateValue(col_nodes[index].axis_nodes.JogOverride, float64(value.Axes.JogOverride))
		UpdateValue(col_nodes[index].axis_nodes.JogSpeed, int64(value.Axes.JogSpeed))
		UpdateValue(col_nodes[index].axis_nodes.CurrentLoad, float64(value.Axes.CurrentLoad))
		UpdateValue(col_nodes[index].axis_nodes.CurrentLoadPercent, float64(value.Axes.CurrentLoadPercent))
		var sv_loads []string
		for k, v := range value.Axes.ServoLoads {
			sv_loads = append(sv_loads, fmt.Sprintf("%s: %d", k, v))
		}
		sv_loads_str := strings.Join(sv_loads, ", ")
		UpdateValue(col_nodes[index].axis_nodes.ServoLoads, string(sv_loads_str))
		UpdateValue(col_nodes[index].axis_nodes.AxesErr, string(value.Axes.AxesErr))
		//spindle data
		UpdateValue(col_nodes[index].spindle_nodes.SpindleSpeed, int64(value.Spindle.SpindleSpeed))
		UpdateValue(col_nodes[index].spindle_nodes.SpindleSpeedParam, int64(value.Spindle.SpindleSpeedParam))
		var sp_motor_speeds []string
		for k, v := range value.Spindle.SpindleMotorSpeed {
			sp_motor_speeds = append(sp_motor_speeds, fmt.Sprintf("%s: %d", k, v))
		}
		motor_speeds_str := strings.Join(sp_motor_speeds, ", ")
		UpdateValue(col_nodes[index].spindle_nodes.SpindleMotorSpeed, string(motor_speeds_str))
		var sp_loads []string
		for k, v := range value.Spindle.SpindleLoad {
			sp_loads = append(sp_loads, fmt.Sprintf("%s: %d", k, v))
		}
		sp_loads_str := strings.Join(sp_loads, ", ")
		UpdateValue(col_nodes[index].spindle_nodes.SpindleLoad, string(sp_loads_str))
		UpdateValue(col_nodes[index].spindle_nodes.SpindleOverride, int64(value.Spindle.SpindleOverride))
		UpdateValue(col_nodes[index].spindle_nodes.SpindleErr, string(value.Spindle.SpindleErr))
	}
}

func CreateCollectorNodes(data CollectorsData, node_ns *server.NodeNameSpace) {
	node_obj := node_ns.Objects()
	collectors := data.Collectors
	for index := range collectors {
		var col_ns CollectorNodes

		device_folder := GetFolderNode(node_ns, node_obj, data.Collectors[index].Device.Name)
		device_data_folder := GetFolderNode(node_ns, device_folder, "device_data")

		name_val := string(collectors[index].Device.Name)
		col_ns.device_nodes.name = AddVariableNode(node_ns, device_data_folder, "name", name_val)

		address_val := string(collectors[index].Device.Address)
		col_ns.device_nodes.address = AddVariableNode(node_ns, device_data_folder, "address", address_val)

		port_val := int64(collectors[index].Device.Port)
		col_ns.device_nodes.port = AddVariableNode(node_ns, device_data_folder, "port", port_val)

		series_val := string(collectors[index].Device.Series)
		col_ns.device_nodes.series = AddVariableNode(node_ns, device_data_folder, "series", series_val)

		//mode data folder + variables
		mode_folder := GetFolderNode(node_ns, device_folder, "mode_data")

		mode_val := collectors[index].Mode.Mode
		col_ns.mode_nodes.Mode = AddVariableNode(node_ns, mode_folder, "mode", mode_val)

		run_state_val := collectors[index].Mode.RunState
		col_ns.mode_nodes.RunState = AddVariableNode(node_ns, mode_folder, "run_state", run_state_val)

		status_val := collectors[index].Mode.Status
		col_ns.mode_nodes.Status = AddVariableNode(node_ns, mode_folder, "status", status_val)

		shutdowns_val := collectors[index].Mode.Shutdowns
		col_ns.mode_nodes.Shutdowns = AddVariableNode(node_ns, mode_folder, "shutdowns", shutdowns_val)

		hight_speed_val := collectors[index].Mode.HightSpeed
		col_ns.mode_nodes.HightSpeed = AddVariableNode(node_ns, mode_folder, "hight_speed", hight_speed_val)

		axis_motion_val := collectors[index].Mode.AxisMotion
		col_ns.mode_nodes.AxisMotion = AddVariableNode(node_ns, mode_folder, "axis_motion", axis_motion_val)

		mstb_val := collectors[index].Mode.Mstb
		col_ns.mode_nodes.Mstb = AddVariableNode(node_ns, mode_folder, "mstb", mstb_val)

		load_excess_val := collectors[index].Mode.LoadExcess
		col_ns.mode_nodes.LoadExcess = AddVariableNode(node_ns, mode_folder, "load_excess", load_excess_val)

		mode_err_val := string(collectors[index].Mode.ModeErr)
		col_ns.mode_nodes.ModeErr = AddVariableNode(node_ns, mode_folder, "mode_err", mode_err_val)

		//program data folder + variables
		program_folder := GetFolderNode(node_ns, device_folder, "program_data")

		frame_val := string(collectors[index].Program.Frame)
		col_ns.prog_nodes.Frame = AddVariableNode(node_ns, program_folder, "frame", frame_val)

		main_prog_number_val := int64(collectors[index].Program.MainProgNumber)
		col_ns.prog_nodes.MainProgNumber = AddVariableNode(node_ns, program_folder, "main_prog_number", main_prog_number_val)

		sub_prog_number_val := int64(collectors[index].Program.SubProgNumber)
		col_ns.prog_nodes.SubProgNumber = AddVariableNode(node_ns, program_folder, "sub_prog_number", sub_prog_number_val)

		parts_count_val := int64(collectors[index].Program.PartsCount)
		col_ns.prog_nodes.PartsCount = AddVariableNode(node_ns, program_folder, "parts_count", parts_count_val)

		tool_number_val := int64(collectors[index].Program.ToolNumber)
		col_ns.prog_nodes.ToolNumber = AddVariableNode(node_ns, program_folder, "tool_number", tool_number_val)

		frame_number_val := int64(collectors[index].Program.FrameNumber)
		col_ns.prog_nodes.FrameNumber = AddVariableNode(node_ns, program_folder, "frame_number", frame_number_val)

		frame_err_val := string(collectors[index].Axes.AxesErr)
		col_ns.prog_nodes.PrgErr = AddVariableNode(node_ns, program_folder, "prg_err", frame_err_val)

		//axes data folder + variables
		axes_folder := GetFolderNode(node_ns, device_folder, "axes_data")

		feedrate_val := int64(collectors[index].Axes.FeedRate)
		col_ns.axis_nodes.FeedRate = AddVariableNode(node_ns, axes_folder, "feedrate", feedrate_val)

		feed_override_val := int64(collectors[index].Axes.FeedOverride)
		col_ns.axis_nodes.FeedOverride = AddVariableNode(node_ns, axes_folder, "feed_override", feed_override_val)

		jog_override_val := float64(collectors[index].Axes.JogOverride)
		col_ns.axis_nodes.JogOverride = AddVariableNode(node_ns, axes_folder, "jog_override", jog_override_val)

		jog_speed_val := int64(collectors[index].Axes.JogSpeed)
		col_ns.axis_nodes.JogSpeed = AddVariableNode(node_ns, axes_folder, "jog_speed", jog_speed_val)

		current_load_val := float64(collectors[index].Axes.CurrentLoad)
		col_ns.axis_nodes.CurrentLoad = AddVariableNode(node_ns, axes_folder, "current_load", current_load_val)

		current_load_percent_val := float64(collectors[index].Axes.CurrentLoadPercent)
		col_ns.axis_nodes.CurrentLoadPercent = AddVariableNode(node_ns, axes_folder, "current_load_percent", current_load_percent_val)

		var sv_loads []string
		for k, v := range collectors[index].Axes.ServoLoads {
			sv_loads = append(sv_loads, fmt.Sprintf("%s: %d", k, v))
		}
		sv_loads_str := strings.Join(sv_loads, ", ")
		col_ns.axis_nodes.ServoLoads = AddVariableNode(node_ns, axes_folder, "servo_loads", sv_loads_str)

		axes_err_val := string(collectors[index].Axes.AxesErr)
		col_ns.axis_nodes.AxesErr = AddVariableNode(node_ns, axes_folder, "axes_err", axes_err_val)

		//spindle data folder + variables
		spindle_folder := GetFolderNode(node_ns, device_folder, "spindle_data")

		spindle_speed_val := int64(collectors[index].Spindle.SpindleSpeed)
		col_ns.spindle_nodes.SpindleSpeed = AddVariableNode(node_ns, spindle_folder, "spindle_speed", spindle_speed_val)

		spindle_param_speed_val := int64(collectors[index].Spindle.SpindleSpeedParam)
		col_ns.spindle_nodes.SpindleSpeedParam = AddVariableNode(node_ns, spindle_folder, "spindle_param_speed", spindle_param_speed_val)

		var sp_motor_speeds []string
		for k, v := range collectors[index].Spindle.SpindleMotorSpeed {
			sp_motor_speeds = append(sp_motor_speeds, fmt.Sprintf("%s: %d", k, v))
		}
		motor_speeds_str := strings.Join(sp_motor_speeds, ", ")
		col_ns.spindle_nodes.SpindleMotorSpeed = AddVariableNode(node_ns, spindle_folder, "spindle_motor_speed", motor_speeds_str)

		var sp_loads []string
		for k, v := range collectors[index].Spindle.SpindleLoad {
			sp_loads = append(sp_loads, fmt.Sprintf("%s: %d", k, v))
		}
		sp_loads_str := strings.Join(sp_loads, ", ")
		col_ns.spindle_nodes.SpindleLoad = AddVariableNode(node_ns, spindle_folder, "spindle_load", sp_loads_str)

		spindle_override_val := int64(collectors[index].Spindle.SpindleOverride)
		col_ns.spindle_nodes.SpindleOverride = AddVariableNode(node_ns, spindle_folder, "spindle_override", spindle_override_val)

		spindle_err_val := string(collectors[index].Spindle.SpindleErr)
		col_ns.spindle_nodes.SpindleErr = AddVariableNode(node_ns, spindle_folder, "spindle_err", spindle_err_val)

		col_nodes = append(col_nodes, col_ns)
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
