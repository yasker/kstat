package main

import (
	"fmt"
	"os"
	"sort"
	"text/template"

	"code.cloudfoundry.org/bytefmt"
	aurora "github.com/logrusorgru/aurora/v3"
	"golang.org/x/crypto/ssh/terminal"
)

func printMetrics(metrics map[string]*ClusterMetric, outputTmpl *template.Template, lineCounter *int) {
	instanceList := []string{}

	// choose a random one to get the instance list
	for _, mi := range metrics {
		for inst := range *mi {
			instanceList = append(instanceList, inst)
		}
		break
	}

	if len(instanceList) == 0 {
		fmt.Println("No data available")
		return
	}

	*lineCounter += len(instanceList)

	sort.Strings(instanceList)

	header := fmt.Sprintf("%20s : %24s | %7s | %15s | %15s\n",
		"", "----------cpu----------", "--mem--", "-----disk-----", "---network---")
	subheader := fmt.Sprintf("%20s : %4s %4s %4s %4s %4s | %7s | %7s %7s | %7s %7s\n",
		"instance", "usr", "sys", "idl", "wai", "stl", "avail", "read", "write", "recv", "send")
	//output := ""
	for _, inst := range instanceList {
		/*
			output += fmt.Sprintf("%20s : %s %s %s %s %s | %s | %s %s | %s %s\n",
				inst,
				colorCPU("%4d", (*metrics[MetricNameCPUUser])[inst].Average),
				colorCPU("%4d", (*metrics[MetricNameCPUSystem])[inst].Average),
				colorCPU("%4d", (*metrics[MetricNameCPUIdle])[inst].Average),
				colorCPU("%4d", (*metrics[MetricNameCPUWait])[inst].Average),
				colorCPU("%4d", (*metrics[MetricNameCPUSteal])[inst].Average),
				colorSize("%7s", bytefmt.ByteSize(uint64((*metrics[MetricNameMemAvailable])[inst].Value))),
				colorSize("%7s", bytefmt.ByteSize(uint64((*metrics[MetricNameDiskRead])[inst].Total))),
				colorSize("%7s", bytefmt.ByteSize(uint64((*metrics[MetricNameDiskWrite])[inst].Total))),
				colorSize("%7s", bytefmt.ByteSize(uint64((*metrics[MetricNameNetworkReceive])[inst].Total))),
				colorSize("%7s", bytefmt.ByteSize(uint64((*metrics[MetricNameNetworkTransmit])[inst].Total))))
		*/
		m := map[string]string{
			"instance":         inst,
			"cpu_user":         colorCPU((*metrics[MetricNameCPUUser])[inst].Average),
			"cpu_system":       colorCPU((*metrics[MetricNameCPUSystem])[inst].Average),
			"cpu_idle":         colorCPU((*metrics[MetricNameCPUIdle])[inst].Average),
			"cpu_wait":         colorCPU((*metrics[MetricNameCPUWait])[inst].Average),
			"cpu_steal":        colorCPU((*metrics[MetricNameCPUSteal])[inst].Average),
			"mem_avail":        colorSize(bytefmt.ByteSize(uint64((*metrics[MetricNameMemAvailable])[inst].Value))),
			"disk_read":        colorSize(bytefmt.ByteSize(uint64((*metrics[MetricNameDiskRead])[inst].Total))),
			"disk_write":       colorSize(bytefmt.ByteSize(uint64((*metrics[MetricNameDiskWrite])[inst].Total))),
			"network_receive":  colorSize(bytefmt.ByteSize(uint64((*metrics[MetricNameNetworkReceive])[inst].Total))),
			"network_transmit": colorSize(bytefmt.ByteSize(uint64((*metrics[MetricNameNetworkTransmit])[inst].Total))),
		}
		if err := outputTmpl.Execute(os.Stdout, m); err != nil {
			fmt.Printf("failed to parse for instance %v\n", inst)
		}
	}

	if needHeader(lineCounter) {
		fmt.Print(header)
		fmt.Print(subheader)
	}
	//fmt.Print(output)
}

func colorCPU(percentage int64) string {
	format := "%4d"
	if percentage <= 0 {
		return aurora.Sprintf(aurora.Gray(10, format), percentage)
	} else if percentage < 33 {
		return aurora.Sprintf(aurora.Red(format), percentage)
	} else if percentage < 66 {
		return aurora.Sprintf(aurora.Yellow(format), percentage)
	} else if percentage < 99 {
		return aurora.Sprintf(aurora.Green(format), percentage)
	}
	return aurora.Sprintf(aurora.BrightWhite(format), percentage)
}

func colorSize(byteString string) string {
	format := "%7s"
	if byteString == "0B" {
		return aurora.Sprintf(aurora.Gray(10, format), byteString)
	}

	unit := byteString[len(byteString)-1]
	switch unit {
	case 'B':
		return aurora.Sprintf(aurora.Red(format), byteString)
	case 'K':
		return aurora.Sprintf(aurora.Yellow(format), byteString)
	case 'M':
		return aurora.Sprintf(aurora.Green(format), byteString)
	}
	return aurora.Sprintf(aurora.BrightWhite(format), byteString)
}

func needHeader(lineCounter *int) bool {
	_, termHeight, err := terminal.GetSize(0)
	if err != nil {
		//logrus.Warnf("Failed to get terminal size: %v", err)
		return true
	}
	// count in the header
	if *lineCounter >= termHeight-2 {
		*lineCounter = 0
	}
	if *lineCounter == 0 {
		return true
	}
	return false
}
