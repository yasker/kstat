package client

import (
	"fmt"
	"sort"
	"strings"

	"code.cloudfoundry.org/bytefmt"
	aurora "github.com/logrusorgru/aurora/v3"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/yasker/kstat/pkg/types"
)

const (
	MetricsOutputSummaryKey = "SUMMARY"
)

func (c *Client) printMetrics(metrics map[string]*types.ClusterMetric, lineCounter *int) {
	instanceMap := map[string]map[string]struct{}{}

	for _, mi := range metrics {
		for inst, m := range mi.InstanceMetrics {
			if instanceMap[inst] == nil {
				instanceMap[inst] = map[string]struct{}{}
			}
			devMap := instanceMap[inst]
			for dev := range (*m).DeviceMetrics {
				devMap[dev] = struct{}{}
			}
		}
	}
	instanceList := []string{}
	instanceDeviceList := map[string][]string{}
	for k := range instanceMap {
		instanceList = append(instanceList, k)
		devList := []string{}
		for d := range instanceMap[k] {
			devList = append(devList, d)
		}
		sort.Strings(devList)
		instanceDeviceList[k] = devList
	}

	if len(instanceList) == 0 {
		fmt.Println("No data available")
		return
	}

	sort.Strings(instanceList)

	output := &strings.Builder{}
	if needHeader(lineCounter) {
		hm := map[string]string{
			"instance": "instance",
		}
		for k, c := range c.metricFormatMap {
			hm[k] = c.Shorthand
		}
		if err := c.headerTemplate.Execute(output, hm); err != nil {
			fmt.Printf("failed to parse for header\n")
		}
	}

	*lineCounter += len(instanceList)

	for _, inst := range instanceList {
		// instance -> instance device -> metrics
		// special key SUMMARY stored the summarized metrics
		mc := map[string]map[string]string{}
		mc[MetricsOutputSummaryKey] = map[string]string{}
		mc[MetricsOutputSummaryKey]["instance"] = inst
		for k, m := range metrics {
			cfg, exist := c.metricFormatMap[k]
			if !exist {
				fmt.Printf("BUG: shouldn't have undefined metric: %v\n", k)
				continue
			}
			value := ""
			if m != nil && m.InstanceMetrics[inst] != nil {
				switch cfg.ValueType {
				case types.ValueTypeCPU:
					value = colorCPU(m.InstanceMetrics[inst].Average)
				case types.ValueTypeSize:
					value = colorSize(bytefmt.ByteSize(uint64(m.InstanceMetrics[inst].Total)))
				default:
					fmt.Printf("Unknown value type %v for %v\n", cfg.ValueType, k)
				}
				if c.ShowDevices {
					devValue := ""
					for devName, devMetrics := range m.InstanceMetrics[inst].DeviceMetrics {
						switch cfg.ValueType {
						case types.ValueTypeCPU:
							devValue = colorCPU(devMetrics)
						case types.ValueTypeSize:
							devValue = colorSize(bytefmt.ByteSize(uint64(devMetrics)))
						default:
							fmt.Printf("Unknown value type %v for %v\n", cfg.ValueType, k)
						}
						if mc[devName] == nil {
							mc[devName] = map[string]string{}
							mc[devName]["instance"] = devName
						}
						mc[devName][k] = devValue
					}
				}
			} else {
				switch cfg.ValueType {
				case types.ValueTypeCPU:
					value = colorNA(types.ValueTypeCPUFormat)
				case types.ValueTypeSize:
					value = colorNA(types.ValueTypeSizeFormat)
				default:
					fmt.Printf("Unknown value type %v for %v\n", cfg.ValueType, k)
				}
			}
			mc[MetricsOutputSummaryKey][k] = value
		}

		if err := c.outputTemplate.Execute(output, mc[MetricsOutputSummaryKey]); err != nil {
			fmt.Printf("failed to parse for instance %v\n", inst)
		}
		if c.ShowDevices {
			for _, dName := range instanceDeviceList[inst] {
				for k, cfg := range c.metricFormatMap {
					_, exists := mc[dName][k]
					if !exists {
						value := ""
						switch cfg.ValueType {
						case types.ValueTypeCPU:
							value = fmt.Sprintf(types.ValueTypeCPUFormat, "")
						case types.ValueTypeSize:
							value = fmt.Sprintf(types.ValueTypeSizeFormat, "")
						default:
							fmt.Printf("Unknown value type %v for %v\n", cfg.ValueType, k)
						}
						mc[dName][cfg.Name] = value
					}
				}
				if err := c.outputTemplate.Execute(output, mc[dName]); err != nil {
					fmt.Printf("failed to parse for instance device %v\n", dName)
				}
			}
		}
	}

	fmt.Print(output.String())
}

func colorCPU(percentage int64) string {
	format := "%5d"
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
	format := "%8s"
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

func colorNA(format string) string {
	return aurora.Sprintf(aurora.BrightRed(format), "NA")
}

func needHeader(lineCounter *int) bool {
	_, termHeight, err := terminal.GetSize(0)
	if err != nil {
		//failed to get terminal size
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

func (c *Client) printTop(metrics map[string]*types.ClusterMetric) {
	lineCounter := 0
	c.printMetrics(metrics, &lineCounter)
}
