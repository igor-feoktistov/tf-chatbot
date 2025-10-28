PROJECT := tf-chatbot
REGISTRY ?= docker-kubernetes-tools.repo.east1.ncloud.netapp.com
IMAGE := $(REGISTRY)/$(PROJECT)
GOFMT_FILES?=$$(find . -name '*.go' |grep -v vendor)
OSFLAG=$(shell go env GOHOSTOS)

VERSION ?= 1.0.0

default: build

build:
	# Build linux-amd64 binaries
	#GOOS=linux GOARCH=amd64 go build -o tf-chatbot ./cmd/...
	# Build darwin-amd64 binaries
	GOOS=darwin GOARCH=amd64 go build -o tf-chatbot ./cmd/...

image:
	docker build . -t $(IMAGE):$(VERSION)

push:
	docker push $(IMAGE):$(VERSION)

fmt:
	# Format the code
	gofmt -w $(GOFMT_FILES)

PHONY: build fmt image push
