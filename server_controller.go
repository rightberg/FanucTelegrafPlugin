// Copyright 2018-2020 opcua authors. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"flag"
	"log"
	"os"
	"os/signal"
	"strconv"
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
}

type ProgramNodes struct {
	Frame          *server.Node
	MainProgNumber *server.Node
	SubProgNumber  *server.Node
	PartsCount     *server.Node
	ToolNumber     *server.Node
	FrameNumber    *server.Node
}

type AxisNodes struct {
	FeedRate           *server.Node
	FeedOverride       *server.Node
	JogOverride        *server.Node
	JogSpeed           *server.Node
	CurrentLoad        *server.Node
	CurrentLoadPercent *server.Node
	ServoLoads         *server.Node
}

type CollectorNodes struct {
	address    *server.Node
	port       *server.Node
	series     *server.Node
	mode_nodes ModeNodes
	prog_nodes ProgramNodes
	axis_nodes AxisNodes
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

func inicialize() {
	flag.BoolVar(&debug.Enable, "debug", false, "enable debug logging")
	flag.Parse()
	log.SetFlags(0)

	var opts []server.Option

	// Set your security options.
	opts = append(opts,
		server.EnableSecurity("None", ua.MessageSecurityModeNone),
	)

	// Set your user authentication options.
	opts = append(opts,
		server.EnableAuthMode(ua.UserTokenTypeAnonymous),
	)

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
	nodeNS.Objects().SetDescription("Fanuc devices data", "Data from fanuc collector")
	_node_ns = nodeNS
	log.Printf("Node Namespace added at index %d", nodeNS.ID())

	nns_obj := nodeNS.Objects()
	root_obj_node.AddRef(nns_obj, id.HasComponent, true)

	CreateCollectorNodes(collectors_data, nodeNS)
}

func AddVariableNode(node_ns *server.NodeNameSpace, node *server.Node, name string, value any) *server.Node {
	variable := node_ns.AddNewVariableNode(name, value)
	variable.SetDescription("Description", name)
	node.AddRef(variable, id.HasComponent, true)
	return variable
}

func GetFolderNode(node_id uint16, key string, name string) *server.Node {
	folder_id := ua.NewStringNodeID(node_id, key)
	attributes := map[ua.AttributeID]*ua.DataValue{
		ua.AttributeIDNodeClass:  server.DataValueFromValue(uint32(ua.NodeClassObject)),
		ua.AttributeIDBrowseName: server.DataValueFromValue(attrs.BrowseName(name)),
	}
	folder := server.NewNode(folder_id, attributes, nil, nil)
	folder.SetDescription("Description", "ru")
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
		UpdateValue(col_nodes[index].address, string(value.Device.Address))
		UpdateValue(col_nodes[index].port, int64(value.Device.Port))
		UpdateValue(col_nodes[index].series, string(value.Device.Series))
		//mode data
		UpdateValue(col_nodes[index].mode_nodes.Mode, string(value.Mode.Mode))
		UpdateValue(col_nodes[index].mode_nodes.RunState, string(value.Mode.RunState))
		UpdateValue(col_nodes[index].mode_nodes.Status, string(value.Mode.Status))
		UpdateValue(col_nodes[index].mode_nodes.Shutdowns, string(value.Mode.Shutdowns))
		UpdateValue(col_nodes[index].mode_nodes.HightSpeed, string(value.Mode.HightSpeed))
		UpdateValue(col_nodes[index].mode_nodes.AxisMotion, string(value.Mode.AxisMotion))
		UpdateValue(col_nodes[index].mode_nodes.Mstb, string(value.Mode.LoadExcess))
		UpdateValue(col_nodes[index].mode_nodes.LoadExcess, string(value.Mode.LoadExcess))
		//program data
		UpdateValue(col_nodes[index].prog_nodes.Frame, string(value.Program.Frame))
		UpdateValue(col_nodes[index].prog_nodes.MainProgNumber, int64(value.Program.MainProgNumber))
		UpdateValue(col_nodes[index].prog_nodes.SubProgNumber, int64(value.Program.SubProgNumber))
		UpdateValue(col_nodes[index].prog_nodes.PartsCount, int64(value.Program.PartsCount))
		UpdateValue(col_nodes[index].prog_nodes.ToolNumber, int64(value.Program.ToolNumber))
		UpdateValue(col_nodes[index].prog_nodes.FrameNumber, int64(value.Program.FrameNumber))
		//axis data
		UpdateValue(col_nodes[index].axis_nodes.FeedRate, int64(value.Axes.FeedRate))
		UpdateValue(col_nodes[index].axis_nodes.FeedOverride, int64(value.Axes.FeedOverride))
		UpdateValue(col_nodes[index].axis_nodes.JogOverride, float64(value.Axes.JogOverride))
		UpdateValue(col_nodes[index].axis_nodes.JogSpeed, int64(value.Axes.JogSpeed))
		UpdateValue(col_nodes[index].axis_nodes.CurrentLoad, float64(value.Axes.CurrentLoad))
		UpdateValue(col_nodes[index].axis_nodes.CurrentLoadPercent, float64(value.Axes.CurrentLoadPercent))
	}
}

func CreateCollectorNodes(data CollectorsData, node_ns *server.NodeNameSpace) {
	node_obj := node_ns.Objects()
	collectors := data.Collectors
	for index := range collectors {
		var str_index = strconv.Itoa(index)
		var col_ns CollectorNodes
		device_folder := GetFolderNode(node_ns.ID(), "device "+str_index, "Device "+str_index)
		node_ns.AddNode(device_folder)
		node_obj.AddRef(device_folder, id.HasComponent, true)

		address_val := string(collectors[index].Device.Address)
		col_ns.address = AddVariableNode(node_ns, device_folder, "address", address_val)

		port_val := int64(collectors[index].Device.Port)
		col_ns.port = AddVariableNode(node_ns, device_folder, "port", port_val)

		series_val := string(collectors[index].Device.Series)
		col_ns.series = AddVariableNode(node_ns, device_folder, "series", series_val)

		//mode data folder + variables
		mode_folder := GetFolderNode(node_ns.ID(), "mode_data ", "Mode Data ")
		node_ns.AddNode(mode_folder)
		device_folder.AddRef(mode_folder, id.HasComponent, true)

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

		//program data folder + variables
		program_folder := GetFolderNode(node_ns.ID(), "program_data", "Program Data")
		node_ns.AddNode(program_folder)
		device_folder.AddRef(program_folder, id.HasComponent, true)

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

		//axes data folder + variables
		axes_folder := GetFolderNode(node_ns.ID(), "axes_data", "Axes Data")
		node_ns.AddNode(axes_folder)
		device_folder.AddRef(axes_folder, id.HasComponent, true)

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
