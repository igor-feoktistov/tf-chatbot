PROJECT := tf-chatbot
GOFMT_FILES?=$$(find . -name '*.go' |grep -v vendor)
OSFLAG=$(shell go env GOHOSTOS)

VERSION ?= 1.0.3

default: build

build:
	# Build linux-amd64 binaries
	#GOOS=linux GOARCH=amd64 go build -o tf-chatbot ./cmd/...
	# Build darwin-amd64 binaries
	GOOS=darwin GOARCH=amd64 go build -o tf-chatbot ./cmd/...

fmt:
	# Format the code
	gofmt -w $(GOFMT_FILES)

PHONY: build fmt
