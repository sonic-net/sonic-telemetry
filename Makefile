all: precheck deps patch telemetry
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

.PHONY : all precheck deps patch telemetry clean cleanall check install deinstall

ifdef DEBUG
	GOFLAGS += -gcflags="all=-N -l"
endif

all: deps patch telemetry $(TELEMETRY_TEST_BIN)

precheck:
	$(shell mkdir -p $(BUILD_DIR))

deps: $(BUILD_DIR)/.deps

$(BUILD_DIR)/.deps:
	touch $(BUILD_DIR)/.deps
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/openconfig/gnmi/cli; cd $(GO_DEP_PATH)/src/github.com/openconfig/gnmi/cli; \
git checkout 89b2bf29312cda887da916d0f3a32c1624b7935f 2>/dev/null ; true; \
	GOPATH=$(GO_DEP_PATH) $(GO) get -u  github.com/Workiva/go-datastructures/queue; cd $(GO_DEP_PATH)/src/github.com/Workiva/go-datastructures; \
git checkout f07cbe3f82ca2fd6e5ab94afce65fe43319f675f 2>/dev/null ; true; \
GOPATH=$(GO_DEP_PATH) $(GO) install -v -gcflags "-N -l" $(GO_DEP_PATH)/src/github.com/Workiva/go-datastructures
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/openconfig/goyang; cd $(GO_DEP_PATH)/src/github.com/openconfig/goyang; \
git checkout 064f9690516f4f72db189f4690b84622c13b7296 2>/dev/null ; true; \
GOPATH=$(GO_DEP_PATH) $(GO) install -v -gcflags "-N -l" $(GO_DEP_PATH)/src/github.com/openconfig/goyang
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/openconfig/ygot/ygot; cd $(GO_DEP_PATH)/src/github.com/openconfig/ygot/ygot; \
git checkout b14560776567988c832f9685af6f0e695ee95727 2>/dev/null ; true; \
GOPATH=$(GO_DEP_PATH) $(GO) install -v -gcflags "-N -l" $(GO_DEP_PATH)/src/github.com/openconfig/ygot/ygot
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/golang/glog; cd $(GO_DEP_PATH)/src/github.com/golang/glog; \
git checkout 23def4e6c14b4da8ac2ed8007337bc5eb5007998 2>/dev/null ; true; \
GOPATH=$(GO_DEP_PATH) $(GO) install -v -gcflags "-N -l" $(GO_DEP_PATH)/src/github.com/golang/glog
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/go-redis/redis; cd $(GO_DEP_PATH)/src/github.com/go-redis/redis; \
git checkout d19aba07b47683ef19378c4a4d43959672b7cec8 2>/dev/null ; true; \
GOPATH=$(GO_DEP_PATH) $(GO) install -v -gcflags "-N -l" $(GO_DEP_PATH)/src/github.com/go-redis/redis
	GOPATH=$(GO_DEP_PATH) $(GO) get -u  github.com/c9s/goprocinfo/linux; cd $(GO_DEP_PATH)/src/github.com/c9s/goprocinfo/linux; \
git checkout 0b2ad9ac246b05c4f5750721d0c4d230888cac5e 2>/dev/null ; true; \
GOPATH=$(GO_DEP_PATH) $(GO) install -v -gcflags "-N -l" $(GO_DEP_PATH)/src/github.com/c9s/goprocinfo/linux
	GOPATH=$(GO_DEP_PATH) $(GO) get -u  github.com/golang/protobuf/proto; cd $(GO_DEP_PATH)/github.com/golang/protobuf/proto; \
git checkout ed6926b37a637426117ccab59282c3839528a700 2>/dev/null ; true; \
GOPATH=$(GO_DEP_PATH) $(GO) install -v -gcflags "-N -l" $(GO_DEP_PATH)/src/github.com/golang/protobuf/proto
	GOPATH=$(GO_DEP_PATH) $(GO) get -u  github.com/openconfig/gnmi/proto/gnmi; cd $(GO_DEP_PATH)/github.com/openconfig/gnmi/proto/gnmi; \
git checkout 89b2bf29312cda887da916d0f3a32c1624b7935f 2>/dev/null ; true; \
GOPATH=$(GO_DEP_PATH) $(GO) install -v -gcflags "-N -l" $(GO_DEP_PATH)/src/github.com/openconfig/gnmi/proto/gnmi
	GOPATH=$(GO_DEP_PATH) $(GO) get -u  golang.org/x/net/context
	GOPATH=$(GO_DEP_PATH) $(GO) get -u  google.golang.org/grpc
	GOPATH=$(GO_DEP_PATH) $(GO) get -u google.golang.org/grpc/credentials
	GOPATH=$(GO_DEP_PATH) $(GO) get -u gopkg.in/go-playground/validator.v9
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/gorilla/mux; cd $(GO_DEP_PATH)/src/github.com/gorilla/mux; \
git checkout 49c01487a141b49f8ffe06277f3dca3ee80a55fa 2>/dev/null ; true; \
GOPATH=$(GO_DEP_PATH) $(GO) install -v -gcflags "-N -l" $(GO_DEP_PATH)/src/github.com/gorilla/mux
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/google/gnxi/utils; cd $(GO_DEP_PATH)/github.com/google/gnxi/utils; \
git checkout 6697a080bc2d3287d9614501a3298b3dcfea06df 2>/dev/null ; true; \
GOPATH=$(GO_DEP_PATH) $(GO) install -v -gcflags "-N -l" $(GO_DEP_PATH)/src/github.com/google/gnxi/utils
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/jipanyang/gnxi/utils/xpath; cd $(GO_DEP_PATH)/github.com/jipanyang/gnxi/utils/xpath; \
git checkout f0a90cca6fd0041625bcce561b71f849c9b65a8d 2>/dev/null ; true; \
GOPATH=$(GO_DEP_PATH) $(GO) install -v -gcflags "-N -l" $(GO_DEP_PATH)/src/github.com/jipanyang/gnxi/utils/xpath
	GOPATH=$(GO_DEP_PATH) $(GO) get -u github.com/jipanyang/gnmi/client/gnmi; cd $(GO_DEP_PATH)/github.com/jipanyang/gnmi/client/gnmi; \
git checkout cb4d464fa018c29eadab98281448000bee4dcc3d 2>/dev/null ; true; \
GOPATH=$(GO_DEP_PATH) $(GO) install -v -gcflags "-N -l" $(GO_DEP_PATH)/src/github.com/jipanyang/gnmi/client/gnmi


patch: $(BUILD_DIR)/.patched

$(BUILD_DIR)/.patched:
	touch $(BUILD_DIR)/.patched
	patch -p0 <patches/gnmi_cli.all.patch

telemetry:$(BUILD_DIR)/telemetry $(BUILD_DIR)/dialout_client_cli $(BUILD_DIR)/gnmi_get $(BUILD_DIR)/gnmi_set $(BUILD_DIR)/gnmi_cli

$(BUILD_DIR)/telemetry:src/telemetry/telemetry.go
	@echo "Building $@"
	GOPATH=$(GOPATH) $(GO) build $(GOFLAGS) -o $@ $^
$(BUILD_DIR)/dialout_client_cli:src/dialout/dialout_client_cli/dialout_client_cli.go
	GOPATH=$(GOPATH) $(GO) build $(GOFLAGS) -o $@ $^
$(BUILD_DIR)/gnmi_get:$(BUILD_DIR)/src/github.com/jipanyang/gnxi/gnmi_get/gnmi_get.go
	GOPATH=$(GO_DEP_PATH):$(GOPATH) $(GO) build $(GOFLAGS) -o $@ $^
$(BUILD_DIR)/gnmi_set:$(BUILD_DIR)/src/github.com/jipanyang/gnxi/gnmi_set/gnmi_set.go
	GOPATH=$(GO_DEP_PATH):$(GOPATH) $(GO) build $(GOFLAGS) -o $@ $^
$(BUILD_DIR)/gnmi_cli:$(BUILD_DIR)/src/github.com/openconfig/gnmi
	GOPATH=$(GO_DEP_PATH):$(GOPATH) $(GO) build $(GOFLAGS) -o $@ $^/cmd/gnmi_cli/gnmi_cli.go

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
