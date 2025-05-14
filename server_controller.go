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
	"slices"
	"strings"
	"time"

	"github.com/gopcua/opcua/debug"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/server"
	"github.com/gopcua/opcua/ua"
	"github.com/tidwall/gjson"
)

var (
	certfile = flag.String("cert", "cert.pem", "Путь к certificate файлу")
	keyfile  = flag.String("key", "key.pem", "Путь к PEM Private Key файлу")
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

func GetStrSliceByDot(str string) []string {
	if str == "" {
		return []string{}
	}
	return strings.Split(str, ".")
}

func GetDeviceNodes(device_name string) []*server.Node {
	var result []*server.Node
	var tags_pack string
	for _, device := range config.Devices {
		if device.Name == device_name && len(device.TagsPackName) != 0 {
			tags_pack = device.TagsPackName
		}
	}
	addresses := config.TagPacks[tags_pack]
	node_ns := GetNodeNamespace(_server, fanuc_ns)
	if node_ns != nil {
		for address := range addresses {
			device_address := fmt.Sprintf("ns=%d;s=%s", fanuc_ns, device_name)
			node := GetNodeAtAddress(node_ns, device_address+"/"+address)
			if node != nil {
				result = append(result, node)
			}
		}
	}
	return result
}

func UpdateCollector(json_data string) {
	decode_data, ok := gjson.Parse(json_data).Value().(map[string]interface{})
	if !ok {
		logger.Println("Ошибка приведения: JSON не является map")
		return
	}
	device_name := decode_data["name"].(string)
	node_ns := GetNodeNamespace(_server, fanuc_ns)
	if node_ns == nil {
		logger.Println("(Update node value) некорректный node_ns:", fanuc_ns)
		return
	}
	device_address, exists := device_map[device_name]
	if !exists {
		logger.Println("(Update node value) устройство отсутсвует:", device_name)
		return
	}
	var tag_info []string
	var proc_value any
	var tags_pack_name string
	for index := range config.Devices {
		if device_name == config.Devices[index].Name {
			tags_pack_name = config.Devices[index].TagsPackName
			break
		}
	}
	for tag_full_name, tag_type := range config.TagPacks[tags_pack_name] {
		tag_info = GetStrSliceByDot(tag_full_name)
		switch len(tag_info) {
		case 1:
			proc_value = ConvertValueByType(decode_data[tag_info[0]], tag_type)
		case 2:
			proc_value = ConvertMapValueAtKey(tag_info[1], decode_data[tag_info[0]], tag_type)
		default:
			continue
		}
		UpdateNodeValueAtAddress(node_ns, device_address+"/"+tag_full_name, proc_value)
	}
}

func CreateDeviceNodes(devices []Device, node_ns *server.NodeNameSpace) {
	node_obj := node_ns.Objects()
	device_map = make(map[string]string)
	for index := range devices {
		device_folder := GetFolderNode(node_ns, node_obj, devices[index].Name)
		tags_pack := devices[index].TagsPackName
		if len(tags_pack) != 0 {
			var tag_info []string
			var proc_value any
			pack_tags := config.TagPacks[tags_pack]
			for tag_full_name, tag_type := range pack_tags {
				tag_info = GetStrSliceByDot(tag_full_name)
				if len(tag_info) >= 3 {
					continue
				}
				proc_value = GetZeroValueByTagType(tag_type)
				AddVariableNode(node_ns, device_folder, tag_full_name, proc_value)
			}
		}
		device_map[devices[index].Name] = device_folder.ID().String()
	}
}

func InitServer() {
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
				panic("Проблема генерации cert " + err.Error())
			}
			err = os.WriteFile(cert_pem_path, c, 0644)
			if err != nil {
				panic("Проблема записи файла PEM cert: " + err.Error())
			}
			err = os.WriteFile(key_pem_path, k, 0644)
			if err != nil {
				panic("Проблема записи файла PEM key: " + err.Error())
			}
			der, _ := pem.Decode(c)
			if der == nil {
				panic("Ошибка парсинга PEM данных (cert)")
			}
			err = os.WriteFile(cert_der_path, der.Bytes, 0644)
			if err != nil {
				panic("Проблема записи файла DER cert: " + err.Error())
			}
		}
	}

	if slices.Contains(config.Server.AuthModes, "Certificate") {
		var cert []byte
		logger.Printf("Загрузка cert/key")
		c, err := tls.LoadX509KeyPair(cert_pem_path, key_pem_path)
		if err != nil {
			logger.Println("Ошибка загрузки сертификата: ", err)
		} else {
			pk, ok := c.PrivateKey.(*rsa.PrivateKey)
			if !ok {
				logger.Fatalf("Некорректный private key")
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

func StartServer() {
	if err := _server.Start(context.Background()); err != nil {
		log.Fatalf("Ошибка запуска сервера: %s", err)
	}
	defer _server.Close()
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	defer signal.Stop(sigch)
	<-sigch
}
