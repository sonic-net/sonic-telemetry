module github.com/Azure/sonic-telemetry

go 1.12

require (
	github.com/Azure/sonic-mgmt-common v0.0.0-00010101000000-000000000000
	github.com/Workiva/go-datastructures v1.0.52
	github.com/antchfx/jsonquery v1.1.0 // indirect
	github.com/antchfx/xmlquery v1.2.1 // indirect
	github.com/antchfx/xpath v1.1.2 // indirect
	github.com/bgentry/speakeasy v0.1.0 // indirect
	github.com/c9s/goprocinfo v0.0.0-20191125144613-4acdd056c72d
	github.com/cenkalti/backoff/v4 v4.0.0 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/go-redis/redis v6.15.7+incompatible
	github.com/go-redis/redis/v7 v7.2.0 // indirect
	github.com/gogo/protobuf v1.3.1
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golang/protobuf v1.4.0
	github.com/google/gnxi v0.0.0-20191016182648-6697a080bc2d
	github.com/google/protobuf v3.11.4+incompatible // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0 // indirect
	github.com/jipanyang/gnmi v0.0.0-20180820232453-cb4d464fa018
	github.com/jipanyang/gnxi v0.0.0-20181221084354-f0a90cca6fd0 // indirect
	github.com/kylelemons/godebug v1.1.0
	github.com/msteinert/pam v0.0.0-20190215180659-f29b9f28d6f9
	github.com/openconfig/gnmi v0.0.0-20200617225440-d2b4e6a45802
	github.com/openconfig/gnoi v0.0.0-20191206155121-b4d663a26026
	github.com/openconfig/goyang v0.0.0-20200309174518-a00bece872fc // indirect
	github.com/openconfig/ygot v0.7.1
	github.com/pborman/getopt v0.0.0-20190409184431-ee0cd42419d3 // indirect
	github.com/philopon/go-toposort v0.0.0-20170620085441-9be86dbd762f // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	golang.org/x/crypto v0.0.0-20200302210943-78000ba7a073
	golang.org/x/net v0.0.0-20200301022130-244492dfa37a
	golang.org/x/sys v0.0.0-20200323222414-85ca7c5b95cd // indirect
	google.golang.org/genproto v0.0.0-20200319113533-08878b785e9c // indirect
	google.golang.org/grpc v1.28.0
)

replace github.com/Azure/sonic-mgmt-common => ../sonic-mgmt-common
