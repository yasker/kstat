package main

import (
	"fmt"
	"sort"

	"code.cloudfoundry.org/bytefmt"
	aurora "github.com/logrusorgru/aurora/v3"
	"golang.org/x/crypto/ssh/terminal"
)

func printMetrics(metrics map[MetricType]*ClusterMetric, lineCounter *int) {
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
	output := ""
	for _, inst := range instanceList {
		output += fmt.Sprintf("%20s : %s %s %s %s %s | %s | %s %s | %s %s\n",
			inst,
			colorCPU("%4d", (*metrics[MetricTypeCPUUser])[inst].Average),
			colorCPU("%4d", (*metrics[MetricTypeCPUSystem])[inst].Average),
			colorCPU("%4d", (*metrics[MetricTypeCPUIdle])[inst].Average),
			colorCPU("%4d", (*metrics[MetricTypeCPUWait])[inst].Average),
			colorCPU("%4d", (*metrics[MetricTypeCPUSteal])[inst].Average),
			colorSize("%7s", bytefmt.ByteSize(uint64((*metrics[MetricTypeMemAvailable])[inst].Value))),
			colorSize("%7s", bytefmt.ByteSize(uint64((*metrics[MetricTypeDiskRead])[inst].Total))),
			colorSize("%7s", bytefmt.ByteSize(uint64((*metrics[MetricTypeDiskWrite])[inst].Total))),
			colorSize("%7s", bytefmt.ByteSize(uint64((*metrics[MetricTypeNetworkReceive])[inst].Total))),
			colorSize("%7s", bytefmt.ByteSize(uint64((*metrics[MetricTypeNetworkTransmit])[inst].Total))))
	}

	if needHeader(lineCounter) {
		fmt.Print(header)
		fmt.Print(subheader)
	}
	fmt.Print(output)
}

func colorCPU(format string, percentage int64) string {
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

func colorSize(format, byteString string) string {
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
	if *lineCounter == 0 {
		return true
	}
	// count in the header
	if *lineCounter >= termHeight-2 {
		*lineCounter = 0
		return true
	}
	return false
}
