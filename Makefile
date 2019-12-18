ifeq ($(GOPATH),)
export GOPATH=/tmp/go
endif
export PATH := $(PATH):$(GOPATH)/bin

INSTALL := /usr/bin/install
DBDIR := /var/run/redis/sonic-db/
GO := /usr/local/go/bin/go
TOP_DIR := $(abspath ..)
GO_MGMT_PATH=$(TOP_DIR)/sonic-mgmt-framework
BUILD_DIR := $(GOPATH)/bin
export CVL_SCHEMA_PATH := $(GO_MGMT_PATH)/src/cvl/schema
all: sonic-telemetry

go.mod:
	/usr/local/go/bin/go mod init github.com/Azure/sonic-telemetry
sonic-telemetry: go.mod
	rm -rf cvl
	rm -rf translib
	cp -r ../sonic-mgmt-framework/src/cvl ./
	cp -r ../sonic-mgmt-framework/src/translib ./
	find cvl -name \*\.go -exec sed -i -e 's/\"translib/\"github.com\/Azure\/sonic-telemetry\/translib/g' {} \;
	find translib -name \*\.go -exec sed -i -e 's/\"translib/\"github.com\/Azure\/sonic-telemetry\/translib/g' {} \;
	find cvl -name \*\.go -exec sed -i -e 's/\"cvl/\"github.com\/Azure\/sonic-telemetry\/cvl/g' {} \;
	find translib -name \*\.go -exec sed -i -e 's/\"cvl/\"github.com\/Azure\/sonic-telemetry\/cvl/g' {} \;
	sed -i -e 's/\.\.\/\.\.\/\.\.\/models\/yang/\.\.\/\.\.\/\.\.\/sonic-mgmt-framework\/models\/yang/' translib/ocbinds/oc.go
	sed -i -e 's/\$$GO run \$$BUILD_GOPATH\/src\/github.com\/openconfig\/ygot\/generator\/generator.go/generator/' translib/ocbinds/oc.go
	$(GO) get github.com/openconfig/ygot@724a6b18a9224343ef04fe49199dfb6020ce132a
	$(GO) get github.com/openconfig/goyang@064f9690516f4f72db189f4690b84622c13b7296
	$(GO) install github.com/openconfig/ygot/generator
	$(GO) get -x github.com/golang/glog@23def4e6c14b4da8ac2ed8007337bc5eb5007998
	rm -rf vendor
	$(GO) mod vendor
	ln -s vendor src
	cp -r $(GOPATH)/pkg/mod/github.com/openconfig/goyang@v0.0.0-20190924211109-064f9690516f/* vendor/github.com/openconfig/goyang/
	cp -r $(GOPATH)/pkg/mod/github.com/openconfig/ygot@v0.6.1-0.20190723223108-724a6b18a922/* vendor/github.com/openconfig/ygot/
	chmod -R u+w vendor
	patch -d vendor/github.com/antchfx/jsonquery -p1 < ../sonic-mgmt-framework/patches/jsonquery.patch
	patch -d vendor/github.com/openconfig/goyang -p1 < ../sonic-mgmt-framework/goyang-modified-files/goyang.patch
	patch -d vendor/github.com/openconfig -p1 < ../sonic-mgmt-framework/ygot-modified-files/ygot.patch
	$(GO) generate github.com/Azure/sonic-telemetry/translib/ocbinds
	$(GO) install -mod=vendor github.com/Azure/sonic-telemetry/telemetry
	$(GO) install -mod=vendor github.com/Azure/sonic-telemetry/dialout/dialout_client_cli

	make -C $(GO_MGMT_PATH)/src/cvl/schema
	make -C $(GO_MGMT_PATH)/models
	make -C $(GO_MGMT_PATH)/models/yang
	make -C $(GO_MGMT_PATH)/models/yang/sonic

check:
	sudo mkdir -p ${DBDIR}
	sudo cp ./testdata/database_config.json ${DBDIR}
	sudo mkdir -p /usr/models/yang || true
	sudo find $(GO_MGMT_PATH)/models -name '*.yang' -exec cp {} /usr/models/yang/ \;
	$(GO) test -mod=vendor -v github.com/Azure/sonic-telemetry/gnmi_server
	$(GO) test -mod=vendor -v github.com/Azure/sonic-telemetry/dialout/dialout_client

clean:
	rm -rf cvl
	rm -rf translib
	rm -rf vendor
	chmod -f -R u+w $(GOPATH)/pkg || true
	rm -rf $(GOPATH)
	rm -f src



install:
	$(INSTALL) -D $(BUILD_DIR)/telemetry $(DESTDIR)/usr/sbin/telemetry
	$(INSTALL) -D $(BUILD_DIR)/dialout_client_cli $(DESTDIR)/usr/sbin/dialout_client_cli
# 	$(INSTALL) -D $(BUILD_DIR)/gnmi_get $(DESTDIR)/usr/sbin/gnmi_get
# 	$(INSTALL) -D $(BUILD_DIR)/gnmi_set $(DESTDIR)/usr/sbin/gnmi_set
# 	$(INSTALL) -D $(BUILD_DIR)/gnmi_cli $(DESTDIR)/usr/sbin/gnmi_cli

	mkdir -p $(DESTDIR)/usr/bin/
	cp -r $(GO_MGMT_PATH)/src/cvl/schema $(DESTDIR)/usr/sbin
	mkdir -p $(DESTDIR)/usr/models/yang
	find $(GO_MGMT_PATH)/models -name '*.yang' -exec cp {} $(DESTDIR)/usr/models/yang/ \;

deinstall:
	rm $(DESTDIR)/usr/sbin/telemetry
	rm $(DESTDIR)/usr/sbin/dialout_client_cli
	rm $(DESTDIR)/usr/sbin/gnmi_get
	rm $(DESTDIR)/usr/sbin/gnmi_set


