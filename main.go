package main

import (
	"flag"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/yasker/kstat/pkg/client"
	"github.com/yasker/kstat/pkg/server"
	"github.com/yasker/kstat/pkg/version"
)

const (
	FlagListenAddress    = "listen"
	FlagPrometheusServer = "prometheus-server"
	FlagMetricConfigFile = "metrics-config"

	FlagServer             = "server"
	FlagMetricFormatFile   = "metrics-format"
	FlagHeaderTemplateFile = "header-template"
	FlagOutputTemplateFile = "output-template"
	FlagShowDevices        = "show-devices"
	FlagTop                = "top"
)

func ServerCmd() cli.Command {
	return cli.Command{
		Name: "server",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  FlagListenAddress,
				Usage: "Listening address of the server",
				Value: "localhost:9159",
			},
			cli.StringFlag{
				Name:  FlagPrometheusServer,
				Usage: "Specify the Prometheus Server address",
				Value: "http://localhost:9090",
			},
			cli.StringFlag{
				Name:  FlagMetricConfigFile,
				Usage: "Specify the metric config yaml",
				Value: "cfg/metrics.yaml",
			},
		},
		Action: func(c *cli.Context) {
			if err := startServer(c); err != nil {
				logrus.Fatalf("Error starting kstat server: %v", err)
			}
		},
	}
}

func StatCmd() cli.Command {
	return cli.Command{
		Name: "stat",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  FlagServer,
				Usage: "Specify the kstat server",
				Value: "localhost:9159",
			},
			cli.StringFlag{
				Name:  FlagMetricFormatFile,
				Usage: "Specify the metric format yaml",
				Value: "cfg/metrics-format.yaml",
			},
			cli.StringFlag{
				Name:  FlagHeaderTemplateFile,
				Usage: "Specify the header template file",
				Value: "cfg/header.tmpl",
			},
			cli.StringFlag{
				Name:  FlagOutputTemplateFile,
				Usage: "Specify the output template file",
				Value: "cfg/output.tmpl",
			},
			cli.BoolFlag{
				Name:  FlagShowDevices,
				Usage: "If show devices in the output",
			},
			cli.BoolFlag{
				Name:  FlagTop,
				Usage: "Show in `top` style",
			},
		},
		Action: func(c *cli.Context) {
			if err := stat(c); err != nil {
				logrus.Fatalf("Error running stat: %v", err)
			}
		},
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "kstat"
	app.Version = version.FriendlyVersion()
	app.Usage = "dstat for Kubernetes"
	app.Flags = []cli.Flag{}
	app.Commands = []cli.Command{
		ServerCmd(),
		StatCmd(),
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func startServer(c *cli.Context) error {
	flag.Parse()

	listenAddr := c.String(FlagListenAddress)
	promServer := c.String(FlagPrometheusServer)
	cfgFile := c.String(FlagMetricConfigFile)

	s := server.NewServer(listenAddr, promServer, cfgFile)

	if err := s.Start(); err != nil {
		return err
	}

	return nil
}

func stat(c *cli.Context) error {
	serverAddr := c.String(FlagServer)
	metricFormatFile := c.String(FlagMetricFormatFile)
	headerTmplFile := c.String(FlagHeaderTemplateFile)
	outputTmplFile := c.String(FlagOutputTemplateFile)

	client := client.NewClient(serverAddr, metricFormatFile, headerTmplFile, outputTmplFile)
	client.ShowDevices = c.Bool(FlagShowDevices)
	client.ShowAsTop = c.Bool(FlagTop)
	if err := client.Start(); err != nil {
		return err
	}
	return nil
}
