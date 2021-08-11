#!/bin/bash

set -e

buf lint

protoc pb/v1/protocol.proto --go_out=plugins=grpc:pkg/
