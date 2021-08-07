package main

import (
	"fmt"
	"sort"
	"strings"
	"text/template"

	"code.cloudfoundry.org/bytefmt"
	aurora "github.com/logrusorgru/aurora/v3"
	"golang.org/x/crypto/ssh/terminal"
)

func printMetrics(metrics map[string]*ClusterMetric, cfgMap map[string]*MetricConfig, headerTmpl, outputTmpl *template.Template, lineCounter *int) {
	instanceMap := map[string]struct{}{}

	// choose a random one to get the instance list
	for _, mi := range metrics {
		for inst := range *mi {
			instanceMap[inst] = struct{}{}
		}
	}

	instanceList := []string{}
	for k := range instanceMap {
		instanceList = append(instanceList, k)
	}

	if len(instanceList) == 0 {
		fmt.Println("No data available")
		return
	}

	output := &strings.Builder{}
	if needHeader(lineCounter) {
		hm := map[string]string{
			"instance": "instance",
		}
		for k, c := range cfgMap {
			hm[k] = c.Shorthand
		}
		if err := headerTmpl.Execute(output, hm); err != nil {
			fmt.Printf("failed to parse for header\n")
		}
	}

	*lineCounter += len(instanceList)

	sort.Strings(instanceList)

	for _, inst := range instanceList {
		mc := map[string]string{
			"instance": inst,
		}
		for k, m := range metrics {
			cfg, exist := cfgMap[k]
			if !exist {
				fmt.Printf("BUG: shouldn't have undefined metric: %v\n", k)
				continue
			}
			value := ""
			if m != nil && (*m)[inst] != nil {
				switch cfg.ValueType {
				case ValueTypeCPU:
					value = colorCPU((*m)[inst].Average)
				case ValueTypeSize:
					value = colorSize(bytefmt.ByteSize(uint64((*m)[inst].Total)))
				default:
					fmt.Printf("Unknown value type %v for %v\n", cfg.ValueType, k)
				}
			} else {
				switch cfg.ValueType {
				case ValueTypeCPU:
					value = colorNA(ValueTypeCPUFormat)
				case ValueTypeSize:
					value = colorNA(ValueTypeSizeFormat)
				default:
					fmt.Printf("Unknown value type %v for %v\n", cfg.ValueType, k)
				}
			}
			mc[k] = value
		}
		if err := outputTmpl.Execute(output, mc); err != nil {
			fmt.Printf("failed to parse for instance %v\n", inst)
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
