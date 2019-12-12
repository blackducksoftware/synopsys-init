ifndef REGISTRY
REGISTRY = gcr.io/saas-hub-staging/blackducksoftware
endif

ifdef IMAGE_PREFIX
PREFIX = "$(IMAGE_PREFIX)-"
endif

ifneq (, $(findstring gcr.io,$(REGISTRY)))
PREFIX_CMD = "gcloud"
DOCKER_OPTS = "--"
endif

# Set the release version information
TAG = latest
ifdef IMAGE_TAG
TAG=$(IMAGE_TAG)
endif

CURRENT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
OUTDIR = _output

.PHONY: test

all: local-compile

compile: clean
	docker run -t -i --rm -v ${CURRENT_DIR}:/go/src/github.com/blackducksoftware/synopsys-init/ -w /go/src/github.com/blackducksoftware/synopsys-init -e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=amd64 golang:1.13 go build -o init

container: compile
	docker build -t $(REGISTRY)/$(PREFIX)synopsys-init:$(TAG) .

push: container
	$(PREFIX_CMD) docker $(DOCKER_OPTS) push $(REGISTRY)/$(PREFIX)synopsys-init:$(TAG)

test:
	docker run -t -i --rm -v ${CURRENT_DIR}:/go/src/github.com/blackducksoftware/synopsys-init/ -w /go/src/github.com/blackducksoftware/synopsys-init -e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=amd64 golang:1.13 go test

clean:
	rm -rf init

local-compile: clean
	GO111MODULE=on go build -o init

local-docker-compile: clean
	GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o init

local-push: local-docker-compile
	docker build -t $(REGISTRY)/$(PREFIX)synopsys-init:$(TAG) .
	$(PREFIX_CMD) docker $(DOCKER_OPTS) push $(REGISTRY)/$(PREFIX)synopsys-init:$(TAG)
