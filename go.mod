module github.com/Azure/sonic-telemetry

go 1.12

require (
	github.com/Workiva/go-datastructures v1.0.50
	github.com/c9s/goprocinfo v0.0.0-20191125144613-4acdd056c72d
	github.com/go-redis/redis v6.15.6+incompatible
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.3.2
	github.com/openconfig/gnmi v0.0.0-20190823184014-89b2bf29312c
	golang.org/x/lint v0.0.0-20190313153728-d0100b6bd8b3 // indirect
	golang.org/x/net v0.0.0-20191209160850-c0dbc17a3553
	golang.org/x/tools v0.0.0-20190524140312-2c0ae7006135 // indirect
	google.golang.org/grpc v1.25.1
	honnef.co/go/tools v0.0.0-20190523083050-ea95bdfd59fc // indirect
)

replace github.com/Azure/sonic-mgmt-framework => ../sonic-mgmt-framework
