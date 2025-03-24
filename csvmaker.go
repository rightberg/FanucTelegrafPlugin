package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gopcua/opcua/server"
)

type CSVNode struct {
	TagName     string
	Address     string
	DataType    string
	Description string
}

func GetCSVType(value any) string {
	switch value.(type) {
	case int, int8, int16, int32, int64:
		return "Word"
	case uint, uint8, uint16, uint32, uint64:
		return "Word"
	case float32, float64:
		return "Float"
	case string:
		return "String"
	default:
		return "Default"
	}
}

func GetCSVNodeAtOpcNode(node *server.Node) CSVNode {
	return CSVNode{
		TagName:     node.BrowseName().Name,
		Address:     node.DataType().NodeID.String(),
		DataType:    GetCSVType(node.Value().Value.Value()),
		Description: "Fanuc"}
}

func GetTagsAtOpcNodes(nodes CollectorNodes) []CSVNode {
	csv_nodes := []CSVNode{}
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.address))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.port))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.series))

	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.mode_nodes.Mode))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.mode_nodes.RunState))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.mode_nodes.Status))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.mode_nodes.Shutdowns))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.mode_nodes.HightSpeed))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.mode_nodes.AxisMotion))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.mode_nodes.Mstb))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.mode_nodes.LoadExcess))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.mode_nodes.ModeErr))

	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.prog_nodes.Frame))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.prog_nodes.MainProgNumber))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.prog_nodes.SubProgNumber))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.prog_nodes.PartsCount))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.prog_nodes.ToolNumber))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.prog_nodes.FrameNumber))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.prog_nodes.PrgErr))

	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.axis_nodes.FeedRate))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.axis_nodes.FeedOverride))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.axis_nodes.JogOverride))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.axis_nodes.JogSpeed))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.axis_nodes.CurrentLoad))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.axis_nodes.CurrentLoadPercent))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.axis_nodes.ServoLoads))
	csv_nodes = append(csv_nodes, GetCSVNodeAtOpcNode(nodes.axis_nodes.AxesErr))

	return csv_nodes
}

func MakeCSV(nodes []CSVNode, name string, plugin_path string) {
	if len(nodes) == 0 {
		fmt.Println("Отсутсвуют узлы для записи")
		return
	}
	name += ".csv"

	dir_path := filepath.Join(plugin_path, "CSV")
	file_path := filepath.Join(dir_path, name)

	if _, err := os.Stat(dir_path); os.IsNotExist(err) {
		os.MkdirAll(dir_path, os.ModePerm)
	}

	file, err := os.Create(file_path)
	if err != nil {
		fmt.Println("Ошибка создания файла:", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	headers := []string{"Tag Name", "Address", "Data Type", "Description"}
	err = writer.Write(headers)
	if err != nil {
		fmt.Println("Ошибка записи заголовков:", err)
		return
	}

	for _, node := range nodes {
		row := []string{node.TagName, node.Address, node.DataType, node.Description}
		err = writer.Write(row)
		if err != nil {
			fmt.Println("Ошибка записи строки:", err)
			return
		}
	}
}
