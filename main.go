//go:generate go run pkg/codegen/cleanup/main.go
//go:generate /bin/rm -rf pkg/generated
//go:generate go run pkg/codegen/main.go

package main

import (
	"flag"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/yasker/kstat/pkg/version"
)

var (
	KubeConfig string
)

func main() {
	app := cli.NewApp()
	app.Name = "kstat"
	app.Version = version.FriendlyVersion()
	app.Usage = "dstat for Kubernetes"
	app.Flags = []cli.Flag{}
	app.Action = run

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func run(c *cli.Context) {
	flag.Parse()
	logrus.Info("KStat Started")
}
