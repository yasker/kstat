module github.com/yasker/kstat

go 1.16

replace k8s.io/client-go => k8s.io/client-go v0.18.0

require (
	code.cloudfoundry.org/bytefmt v0.0.0-20210608160410-67692ebc98de
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/common v0.30.0
	github.com/sirupsen/logrus v1.6.0
	github.com/urfave/cli v1.22.2
)
