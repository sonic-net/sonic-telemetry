ifeq ($(GOPATH),)
export GOPATH=/tmp/go
endif

INSTALL := /usr/bin/install
DBDIR := /var/run/redis/sonic-db/

all: sonic-telemetry

go.mod:
	/usr/local/go/bin/go mod init github.com/Azure/sonic-telemetry
sonic-telemetry: go.mod
	/usr/local/go/bin/go install github.com/Azure/sonic-telemetry/telemetry
	/usr/local/go/bin/go install github.com/Azure/sonic-telemetry/dialout/dialout_client_cli

check:
	sudo mkdir -p ${DBDIR}
	sudo cp ./testdata/database_config.json ${DBDIR}
	/usr/local/go/bin/go test -v github.com/Azure/sonic-telemetry/gnmi_server
	/usr/local/go/bin/go test -v github.com/Azure/sonic-telemetry/dialout/dialout_client

install:
	$(INSTALL) -D ${GOPATH}/bin/telemetry $(DESTDIR)/usr/sbin/telemetry
	$(INSTALL) -D ${GOPATH}/bin/dialout_client_cli $(DESTDIR)/usr/sbin/dialout_client_cli

deinstall:
	rm $(DESTDIR)/usr/sbin/telemetry
	rm $(DESTDIR)/usr/sbin/dialout_client_cli

