all: precheck deps telemetry
GO=go
SRC_FILES=$(wildcard ./src/telemetry/*.go)
TOP_DIR := $(abspath ..)
TELEM_DIR := $(abspath .)
GOFLAGS:=
BUILD_DIR=build
GO_DEP_PATH=$(abspath .)/$(BUILD_DIR)
GO_MGMT_PATH=$(TOP_DIR)/sonic-mgmt-framework
GO_SONIC_TELEMETRY_PATH=$(TOP_DIR)
CVL_GOPATH=$(GO_MGMT_PATH):$(GO_MGMT_PATH)/src/cvl/build
GOPATH = $(CVL_GOPATH):$(GO_DEP_PATH):$(GO_MGMT_PATH):$(GO_SONIC_TELEMETRY_PATH):$(TELEM_DIR)

ifdef DEBUG
	GOFLAGS += -gcflags="all=-N -l"
endif

precheck:
	$(shell mkdir -p $(BUILD_DIR))

deps: $(BUILD_DIR)/.deps

$(BUILD_DIR)/.deps:
	touch $(BUILD_DIR)/.deps
	GOPATH=$(GO_DEP_PATH) $(GO) get -u  github.com/Workiva/go-datastructures/queue
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/openconfig/goyang
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/openconfig/ygot/ygot
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/golang/glog
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/go-redis/redis
	GOPATH=$(GO_DEP_PATH) $(GO) get -u  github.com/c9s/goprocinfo/linux
	GOPATH=$(GO_DEP_PATH) $(GO) get -u  github.com/golang/protobuf/proto
	GOPATH=$(GO_DEP_PATH) $(GO) get -u  github.com/openconfig/gnmi/proto/gnmi
	GOPATH=$(GO_DEP_PATH) $(GO) get -u  golang.org/x/net/context
	GOPATH=$(GO_DEP_PATH) $(GO) get -u  google.golang.org/grpc
	GOPATH=$(GO_DEP_PATH) $(GO) get -u google.golang.org/grpc/credentials
	GOPATH=$(GO_DEP_PATH) $(GO) get -u gopkg.in/go-playground/validator.v9
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/gorilla/mux
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/openconfig/goyang
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/openconfig/ygot/ygot
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/antchfx/jsonquery
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/antchfx/xmlquery

telemetry:$(BUILD_DIR)/telemetry

$(BUILD_DIR)/telemetry:
	@echo "Building $@"
	GOPATH=$(GOPATH) $(GO) build $(GOFLAGS) -o $@ $(SRC_FILES)

clean:
	rm -rf $(BUILD_DIR)/telemetry
	make -C  $(GO_MGMT_PATH) clean

cleanall:
	rm -rf $(BUILD_DIR)
	make -C  $(GO_MGMT_PATH) cleanall
