# SONiC-telemetry

## Description
This repository contains implementation for the sonic system telemetry services:
- dial-in mode system telemetry server: `telemetry`
- dial-out mode system telemetry client `dialout_client_cli`

## Getting Started

### Prerequisites

* Install __go__ in your system https://golang.org/doc/install. Requires golang1.8+.
* Install Git
* Install Libpcre3-dev using: sudo apt install -y libpcre3-dev
* Install Customized version of libyang. These can obtained from a local sonic-buildimage folder and installed using the command:

        sudo dpkg -i ${buildimage}/target/debs/stretch/libyang*.deb

    or they can be downloaded from Jenkins and installed individually. Libyang, Libyang-cpp, Libyang-dbg, and Libyang-dev all need to be installed. https://sonic-jenkins.westus2.cloudapp.azure.com/job/generic/job/buildimage-baseimage/lastSuccessfulBuild/artifact/sonic-buildimage/target/debs/buster/
* Install PCRE http://www.pcre.org/ compiled with --enable-unicode-properties. A compiling guide can be found https://mac-dev-env.patrickbougie.com/pcre/

## Installing

There is a test program dialout_server_cli for collecting data from dial-out mode system telemetry client. _Note_: it is for testing purpose only. Only Go is a prerequisite.

    go get -u github.com/Azure/sonic-telemetry/dialout/dialout_server_cli

The binaries will be installed under $GOPATH/bin/, they may be copied to any SONiC switch and run there.

To use the telemetry server or dial-out client you can build SONiC-telemetry into a debian package and install it using the following steps:

    git clone https://github.com/Azure/sonic-mgmt-common.git
    git clone https://github.com/Azure/sonic-telemetry.git

    cd sonic-mgmt-common
    make
    sudo dh_install -psonic-mgmt-common -P/-v

    
    cd sonic-telemetry
    make
    sudo install testdata/database_config.json -t /var/run/redis/sonic-db       <- sometimes the sonic-db folder must be created manually.
    export CVL_SCHEMA_PATH=/usr/sbin/schema     <- This must be performed everytime it is run.


To test that it worked:
    
    ./build/bin/telemetry --help

Steps written with Ubuntu 18.04 in July 2020.

### Running
* See [SONiC gRPC telemetry](./doc/grpc_telemetry.md) for how to run dial-in mode system telemetry server
* See [SONiC telemetry in dial-out mode](./doc/dialout.md) for how to run dial-out mode system telemetry client

### Common Errors
| Error | Solution |
| ----------------------- | ----------------------- |
| sudo install testdata/database_config.json -t /var/run/redis/sonic-db <br /> install: failed to access '/var/run/redis/sonic-db': No such file or directory | Manually create the missing folder (/var/run/redis/sonic-db) and try again |
| ./build/bin/dialout_client_cli -insecure -logtostderr -v 1 <br /> libyang[0]: Unable to use search directory "schema/" (No such file or directory) <br /> panic: runtime error: invalid memory address or nil pointer dereference  <br />[signal SIGSEGV: segmentation violation code=0x1 addr=0x50 pc=0x958402] | Ensure that Ensure that CVL_SCHEMA_PATH is properly exported. export CVL_SCHEMA_PATH=/usr/sbin/schema must be run every time |
| user@build:-/Desktop/sonic$ make -C sonic-telemetry <br /> make: Entering directory '/home/user/Desktop/sonic/sonic-telemetry' <br /> # FIXME temporary workaround for crypto not downloading.. <br />/usr/local/go/bin/go get golang.org/x/crypto/ssh/terminal@e9b2fee46413 <br />go: found golang.org/x/crypto/ssh/terminal in golang.org/x/crypto vO.O.O-20191206172530-e9b2fee46413 <br /> # golang.org/x/sys/unix../../../go/pkg/mod/golang.org/x/sys@v0.0.0-2020061520003 <br /> 2-f1bc736245b1/unix/syscall unix.go: 16: 2: import /home/user/. cache/go - bui Id/08/08624f22b1129d677c1be2d3e3fe2b94a <br />f75a12b875aOc9b4e5c521fe24ff51e-d: reading input: EOF <br />Makefile: 33: recipe for target 'vendor/. done' failed <br />make: *** [vendor/. done] Error 2 <br /> make: Leaving directory '/home/user/Desktop/sonic/sonic-telemetry' | Package cache is corrupted. Delete $HOME/.cache/go-build and try again. If it still fails, delete /tmp/go/pkg directory as well.




## Need Help?

For general questions, setup help, or troubleshooting:
- [sonicproject on Google Groups](https://groups.google.com/d/forum/sonicproject)

For bug reports or feature requests, please open an Issue.

## Contribution guide

See the [contributors guide](https://github.com/Azure/SONiC/blob/gh-pages/CONTRIBUTING.md) for information about how to contribute.

### GitHub Workflow

We're following basic GitHub Flow. If you have no idea what we're talking about, check out [GitHub's official guide](https://guides.github.com/introduction/flow/). Note that merge is only performed by the repository maintainer.

Guide for performing commits:

* Isolate each commit to one component/bugfix/issue/feature
* Use a standard commit message format:

>     [component/folder touched]: Description intent of your changes
>
>     [List of changes]
>
> 	  Signed-off-by: Your Name your@email.com

For example:

>     swss-common: Stabilize the ConsumerTable
>
>     * Fixing autoreconf
>     * Fixing unit-tests by adding checkers and initialize the DB before start
>     * Adding the ability to select from multiple channels
>     * Health-Monitor - The idea of the patch is that if something went wrong with the notification channel,
>       we will have the option to know about it (Query the LLEN table length).
>
>       Signed-off-by: user@dev.null


* Each developer should fork this repository and [add the team as a Contributor](https://help.github.com/articles/adding-collaborators-to-a-personal-repository)
* Push your changes to your private fork and do "pull-request" to this repository
* Use a pull request to do code review
* Use issues to keep track of what is going on
