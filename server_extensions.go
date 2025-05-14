package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/server"
	"github.com/gopcua/opcua/server/attrs"
	"github.com/gopcua/opcua/ua"
)

func GetNodeNamespace(s *server.Server, ns_id int) *server.NodeNameSpace {

	namespace, err := s.Namespace(int(ns_id))
	if err == nil {
		return namespace.(*server.NodeNameSpace)
	}
	return nil
}

func AddVariableNode(node_ns *server.NodeNameSpace, node *server.Node, name string, value any) *server.Node {
	parent_id := node.ID().StringID()
	if parent_id != "" {
		parent_id += "/"
	}
	node_id := ua.NewStringExpandedNodeID(node_ns.ID(), parent_id+name)
	vf, _ := value.(func() *ua.DataValue)
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
	variable := server.NewNode(
		ua.NewNodeIDFromExpandedNodeID(node_id),
		attributes,
		[]*ua.ReferenceDescription{},
		vf,
	)
	variable.SetAttribute(ua.AttributeIDValue, server.DataValueFromValue(value))
	if value != nil && reflect.TypeOf(value).Kind() == reflect.Slice {
		variable.SetAttribute(ua.AttributeIDValueRank, server.DataValueFromValue(int32(1)))
	}
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

func GetNodeAtAddress(node_ns *server.NodeNameSpace, address string) *server.Node {
	node_id, _ := ua.ParseNodeID(address)
	if node_id != nil {
		node := node_ns.Node(node_id)
		if node != nil {
			return node
		}
	}
	return nil
}

func UpdateNodeValue(node *server.Node, value any) {
	if node != nil {
		val := ua.DataValue{
			Value:           ua.MustVariant(value),
			SourceTimestamp: time.Now(),
			EncodingMask:    ua.DataValueValue | ua.DataValueSourceTimestamp,
		}
		node.SetAttribute(ua.AttributeIDValue, &val)

		ns_id := node.ID().Namespace()
		namespace, err := _server.Namespace(int(ns_id))
		if err == nil {
			node_ns := namespace.(*server.NodeNameSpace)
			node_ns.ChangeNotification(node.ID())
		}
	}
}

func GetZeroValueByTagType(tag_type string) any {
	switch tag_type {
	case "bool":
		return false
	case "string":
		return ""
	case "int16":
		return int16(0)
	case "int32":
		return int32(0)
	case "int64":
		return int64(0)
	case "float64":
		return float64(0)
	case "[]int64":
		return make([]int64, 0)
	case "[]float64":
		return make([]float64, 0)
	default:
		return nil
	}
}

func ConvertValueByType(value any, data_type string) any {
	switch data_type {
	case "bool":
		if data, ok := value.(bool); ok {
			return data
		}
	case "int16":
		if data, ok := value.(float64); ok {
			return int16(data)
		}
	case "int32":
		if data, ok := value.(float64); ok {
			return int32(data)
		}
	case "int64":
		if data, ok := value.(float64); ok {
			return int64(data)
		}
	case "float64":
		if data, ok := value.(float64); ok {
			return data
		}
	case "string":
		if data, ok := value.(string); ok {
			return data
		}
	case "[]int64":
		if raw_slice, ok := value.([]interface{}); ok {
			var data []int64
			for _, v := range raw_slice {
				if num, ok := v.(float64); ok {
					data = append(data, int64(num))
				}
			}
			return data
		}
	case "[]float64":
		if raw_slice, ok := value.([]interface{}); ok {
			var data []float64
			for _, v := range raw_slice {
				if num, ok := v.(float64); ok {
					data = append(data, float64(num))
				}
			}
			return data
		}
	}
	return nil
}

func ConvertMapValueAtKey(key string, map_data any, data_type string) any {
	if map_data, ok := map_data.(map[string]interface{}); ok {
		switch data_type {
		case "int64":
			for _key, _value := range map_data {
				if buf_value, ok := _value.(float64); ok {
					if _key == strings.ToUpper(key) {
						return int64(buf_value)
					} else if _key == strings.ToLower(key) {
						return int64(buf_value)
					}
				}
			}
			return int64(0)
		case "float64":
			for _key, _value := range map_data {
				if buf_value, ok := _value.(float64); ok {
					if _key == strings.ToUpper(key) {
						return buf_value
					} else if _key == strings.ToLower(key) {
						return buf_value
					}
				}
			}
			return float64(0)
		}
	}
	return nil
}

func UpdateNodeValueAtAddress(node_ns *server.NodeNameSpace, address string, value any) {
	node_id, _ := ua.ParseNodeID(address)
	if node_id != nil {
		node := node_ns.Node(node_id)
		if node != nil {
			val := ua.DataValue{
				Value:           ua.MustVariant(value),
				SourceTimestamp: time.Now(),
				EncodingMask:    ua.DataValueValue | ua.DataValueSourceTimestamp,
			}
			node.SetAttribute(ua.AttributeIDValue, &val)
			ns_id := node.ID().Namespace()
			namespace, err := _server.Namespace(int(ns_id))
			if err == nil {
				node_ns := namespace.(*server.NodeNameSpace)
				node_ns.ChangeNotification(node.ID())
			}
		}
	}
}

func GetPoliciesOptions(policies map[string]string, available_policies []string, available_sec_modes []string) []server.Option {
	if len(policies) == 0 {
		return nil
	}
	options := []server.Option{}
	var loaded_policies []string
	for policy, mode := range policies {
		policy_access := slices.Contains(available_policies, policy)
		mode_access := slices.Contains(available_sec_modes, mode)
		if policy_access && mode_access {
			merge := policy + mode
			if !slices.Contains(loaded_policies, merge) {
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

func GetAuthModeOptions(auth_modes []string, available_auth_modes []string) []server.Option {
	if len(auth_modes) == 0 {
		return nil
	}
	options := []server.Option{}
	var loaded_auth_modes []string
	for _, mode := range auth_modes {
		auth_access := slices.Contains(available_auth_modes, mode)
		if auth_access {
			if !slices.Contains(loaded_auth_modes, mode) {
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

func GetEndpointOptions(endpoints []ImportEndpoint) []server.Option {
	if len(endpoints) == 0 {
		return nil
	}
	options := []server.Option{}
	var loaded_endpoints []string
	var endpoint string
	var port int
	var merge string
	for _, imp_endpoint := range endpoints {
		endpoint = imp_endpoint.Endpoint
		port = imp_endpoint.Port
		merge = fmt.Sprintf("%s%d", endpoint, port)
		access := !slices.Contains(loaded_endpoints, merge)
		if access {
			options = append(options, server.EndPoint(endpoint, port))
			loaded_endpoints = append(loaded_endpoints, merge)
		}
	}
	if len(options) == 0 {
		return nil
	}
	return options
}

func AddCert(cert_path string) server.Option {
	certDER, err := os.ReadFile(cert_path)
	if err == nil {
		if _, err := x509.ParseCertificate(certDER); err != nil {
			log.Printf("Error parsing DER certificate: %v", err)
		} else {
			return server.Certificate(certDER)
		}
	}
	return nil
}

func AddPK(key_path string) server.Option {
	keyBytes, err := os.ReadFile(key_path)
	if err != nil {
		logger.Fatalf("Failed to read key file: %v", err)
	}

	block, _ := pem.Decode(keyBytes)
	if block == nil {
		logger.Fatalf("Failed to decode PEM block containing the key")
	}

	var privateKey *rsa.PrivateKey

	switch block.Type {
	case "RSA PRIVATE KEY":
		privateKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			log.Fatalf("Failed to parse PKCS1 private key: %v", err)
		}
	case "PRIVATE KEY":
		keyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			log.Fatalf("Failed to parse PKCS8 private key: %v", err)
		}
		var ok bool
		privateKey, ok = keyInterface.(*rsa.PrivateKey)
		if !ok {
			log.Fatalf("Parsed key is not an RSA private key")
		}
	default:
		log.Fatalf("Unknown key type %q", block.Type)
	}

	return server.PrivateKey(privateKey)
}
