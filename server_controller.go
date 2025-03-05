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

var options []server.Option
var _server *server.Server

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

	options = opts
}

func launch() {
	// Start the server
	_server = server.New(options...)
	root_ns, _ := _server.Namespace(0)
	root_obj_node := root_ns.Objects()

	if err := _server.Start(context.Background()); err != nil {
		log.Fatalf("Error starting server, exiting: %s", err)
	}
	defer _server.Close()

	nodeNS := server.NewNodeNameSpace(_server, "NodeNamespace")
	log.Printf("Node Namespace added at index %d", nodeNS.ID())
	nns_obj := nodeNS.Objects()
	root_obj_node.AddRef(nns_obj, id.HasComponent, true)

	var1 := nodeNS.AddNewVariableNode("TestVar1", float32(123.45))
	nns_obj.AddRef(var1, id.HasComponent, true)

	var2Value := "BOBER"
	var2 := nodeNS.AddNewVariableStringNode("TestVar2", var2Value)
	nns_obj.AddRef(var2, id.HasComponent, true)

	var3 := server.NewNode(
		ua.NewNumericNodeID(nodeNS.ID(), 12345),
		map[ua.AttributeID]*ua.DataValue{
			ua.AttributeIDBrowseName: server.DataValueFromValue(attrs.BrowseName("MyBrowseName")),
			ua.AttributeIDNodeClass:  server.DataValueFromValue(uint32(ua.NodeClassVariable)),
		},
		nil,
		func() *ua.DataValue { return server.DataValueFromValue(12.34) },
	)
	nodeNS.AddNode(var3)
	nns_obj.AddRef(var3, id.HasComponent, true)

	// simulate a background process updating the data in the namespace.
	go func() {
		updates := 0
		num := 42
		time.Sleep(time.Second * 10)
		for {
			updates++
			num++
			last_value := var1.Value().Value.Value().(float32)
			last_value += 1
			val := ua.DataValue{
				Value:           ua.MustVariant(last_value),
				SourceTimestamp: time.Now(),
				EncodingMask:    ua.DataValueValue | ua.DataValueSourceTimestamp,
			}
			var1.SetAttribute(ua.AttributeIDValue, &val)

			nodeNS.ChangeNotification(var1.ID())

			time.Sleep(time.Second)
		}
	}()

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	defer signal.Stop(sigch)
	<-sigch
}
