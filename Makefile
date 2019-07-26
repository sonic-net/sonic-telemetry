all: precheck deps telemetry
GO=/usr/local/go/bin/go

TOP_DIR := $(abspath ..)
TELEM_DIR := $(abspath .)
GOFLAGS:=
BUILD_DIR=build
GO_DEP_PATH=$(abspath .)/$(BUILD_DIR)
GO_MGMT_PATH=$(TOP_DIR)/sonic-mgmt-framework
GO_SONIC_TELEMETRY_PATH=$(TOP_DIR)
CVL_GOPATH=$(GO_MGMT_PATH):$(GO_MGMT_PATH)/gopkgs:$(GO_MGMT_PATH)/src/cvl/build
GOPATH = /tmp/go:$(CVL_GOPATH):$(GO_DEP_PATH):$(GO_MGMT_PATH):$(GO_SONIC_TELEMETRY_PATH):$(TELEM_DIR)
INSTALL := /usr/bin/install

.PHONY : all precheck deps telemetry clean cleanall check install deinstall

ifdef DEBUG
	GOFLAGS += -gcflags="all=-N -l"
endif

all: deps telemetry

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

telemetry:$(BUILD_DIR)/telemetry $(BUILD_DIR)/dialout_client_cli $(BUILD_DIR)/gnmi_get $(BUILD_DIR)/gnmi_set# $(BUILD_DIR)/gnmi_cli

$(BUILD_DIR)/telemetry:src/telemetry/telemetry.go
	@echo "Building $@"
	make -C $(GO_MGMT_PATH)
	GOPATH=$(GOPATH) $(GO) build $(GOFLAGS) -o $@ $^
$(BUILD_DIR)/dialout_client_cli:src/dialout/dialout_client_cli/dialout_client_cli.go
	GOPATH=$(GOPATH) $(GO) build $(GOFLAGS) -o $@ $^
$(BUILD_DIR)/gnmi_get:src/gnmi_clients/gnmi_get.go
	GOPATH=$(GOPATH) $(GO) build $(GOFLAGS) -o $@ $^
$(BUILD_DIR)/gnmi_set:src/gnmi_clients/gnmi_set.go
	GOPATH=$(GOPATH) $(GO) build $(GOFLAGS) -o $@ $^
$(BUILD_DIR)/gnmi_cli:src/gnmi_clients/gnmi_cli.go
	GOPATH=$(GOPATH) $(GO) build $(GOFLAGS) -o $@ $^

clean:
	rm -rf $(BUILD_DIR)/telemetry
	make -C  $(GO_MGMT_PATH) clean

cleanall:
	rm -rf $(BUILD_DIR)
	make -C  $(GO_MGMT_PATH) cleanall

check:
	-$(GO) test -v ${GOPATH}/src/gnmi_server

install:
	$(INSTALL) -D $(BUILD_DIR)/telemetry $(DESTDIR)/usr/sbin/telemetry
	$(INSTALL) -D $(BUILD_DIR)/dialout_client_cli $(DESTDIR)/usr/sbin/dialout_client_cli
	$(INSTALL) -D $(BUILD_DIR)/gnmi_get $(DESTDIR)/usr/sbin/gnmi_get
	$(INSTALL) -D $(BUILD_DIR)/gnmi_set $(DESTDIR)/usr/sbin/gnmi_set
	$(INSTALL) -D src/gnmi_clients/gnmi_cli $(DESTDIR)/usr/sbin/gnmi_cli

	mkdir -p $(DESTDIR)/usr/bin/
	cp -r $(GO_MGMT_PATH)/src/cvl/schema $(DESTDIR)/usr/sbin
	cp -r $(GO_MGMT_PATH)/src/cvl/schema $(DESTDIR)/usr/bin

deinstall:
	rm $(DESTDIR)/usr/sbin/telemetry
	rm $(DESTDIR)/usr/sbin/dialout_client_cli
	rm $(DESTDIR)/usr/sbin/gnmi_get
	rm $(DESTDIR)/usr/sbin/gnmi_set
