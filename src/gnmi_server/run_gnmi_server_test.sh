#!/bin/bash
GOPATH=$PWD/../sonic-mgmt-framework:$PWD/../sonic-mgmt-framework/gopkgs:$PWD:$HOME/go go test -v -cover -json gnmi_server | tparse -smallscreen -all

