all: precheck deps telemetry
GO=/usr/local/go/bin/go

TOP_DIR := $(abspath ..)
TELEM_DIR := $(abspath .)
GOFLAGS:=
BUILD_DIR=build
GO_DEP_PATH=$(abspath .)/$(BUILD_DIR)
GO_MGMT_PATH=$(TOP_DIR)/sonic-mgmt-framework
GO_SONIC_TELEMETRY_PATH=$(TOP_DIR)
CVL_GOPATH=$(GO_MGMT_PATH)/gopkgs:$(GO_MGMT_PATH):$(GO_MGMT_PATH)/src/cvl/build
GOPATH = $(CVL_GOPATH):$(GO_DEP_PATH):$(GO_MGMT_PATH):/tmp/go:$(GO_SONIC_TELEMETRY_PATH):$(TELEM_DIR)
INSTALL := /usr/bin/install

SRC_FILES=$(shell find . -name '*.go' | grep -v '_test.go' | grep -v '/tests/')
TEST_FILES=$(wildcard *_test.go)
TELEMETRY_TEST_DIR = $(GO_MGMT_PATH)/build/tests/gnmi_server
TELEMETRY_TEST_BIN = $(TELEMETRY_TEST_DIR)/server.test

.PHONY : all precheck deps telemetry clean cleanall check install deinstall

ifdef DEBUG
	GOFLAGS += -gcflags="all=-N -l"
endif

all: deps telemetry $(TELEMETRY_TEST_BIN)

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
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/google/gnxi/utils
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/jipanyang/gnxi/utils/xpath
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/jipanyang/gnmi/client/gnmi

telemetry:$(BUILD_DIR)/telemetry $(BUILD_DIR)/dialout_client_cli $(BUILD_DIR)/gnmi_get $(BUILD_DIR)/gnmi_set $(BUILD_DIR)/gnmi_cli

$(BUILD_DIR)/telemetry:src/telemetry/telemetry.go
	@echo "Building $@"
	GOPATH=$(GOPATH) $(GO) build $(GOFLAGS) -o $@ $^
$(BUILD_DIR)/dialout_client_cli:src/dialout/dialout_client_cli/dialout_client_cli.go
	GOPATH=$(GOPATH) $(GO) build $(GOFLAGS) -o $@ $^
$(BUILD_DIR)/gnmi_get:src/gnmi_clients/gnmi_get.go
	GOPATH=$(GOPATH) $(GO) build $(GOFLAGS) -o $@ $^
$(BUILD_DIR)/gnmi_set:src/gnmi_clients/gnmi_set.go
	GOPATH=$(GOPATH) $(GO) build $(GOFLAGS) -o $@ $^
$(BUILD_DIR)/gnmi_cli:src/gnmi_clients/src/github.com/openconfig/gnmi
	GOPATH=$(PWD)/src/gnmi_clients:$(GOPATH) $(GO) build $(GOFLAGS) -o $@ github.com/openconfig/gnmi/cmd/gnmi_cli

clean:
	rm -rf $(BUILD_DIR)/telemetry
	rm -rf $(TELEMETRY_TEST_DIR)

cleanall:
	rm -rf $(BUILD_DIR)
	rm -rf $(TELEMETRY_TEST_DIR)

check:
	-$(GO) test -v ${GOPATH}/src/gnmi_server

$(TELEMETRY_TEST_BIN): $(TEST_FILES) $(SRC_FILES)
	GOPATH=$(GOPATH) $(GO) test -c -cover gnmi_server -o $@
	cp -r src/testdata $(TELEMETRY_TEST_DIR)
	cp test/01_create_MyACL1_MyACL2.json $(TELEMETRY_TEST_DIR)
	cp -r $(GO_MGMT_PATH)/src/cvl/schema $(TELEMETRY_TEST_DIR)

install:
	$(INSTALL) -D $(BUILD_DIR)/telemetry $(DESTDIR)/usr/sbin/telemetry
	$(INSTALL) -D $(BUILD_DIR)/dialout_client_cli $(DESTDIR)/usr/sbin/dialout_client_cli
	$(INSTALL) -D $(BUILD_DIR)/gnmi_get $(DESTDIR)/usr/sbin/gnmi_get
	$(INSTALL) -D $(BUILD_DIR)/gnmi_set $(DESTDIR)/usr/sbin/gnmi_set
	$(INSTALL) -D $(BUILD_DIR)/gnmi_cli $(DESTDIR)/usr/sbin/gnmi_cli

	mkdir -p $(DESTDIR)/usr/bin/
	cp -r $(GO_MGMT_PATH)/src/cvl/schema $(DESTDIR)/usr/sbin
	cp -r $(GO_MGMT_PATH)/src/cvl/schema $(DESTDIR)/usr/bin

deinstall:
	rm $(DESTDIR)/usr/sbin/telemetry
	rm $(DESTDIR)/usr/sbin/dialout_client_cli
	rm $(DESTDIR)/usr/sbin/gnmi_get
	rm $(DESTDIR)/usr/sbin/gnmi_set
