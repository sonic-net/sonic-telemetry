package main

import (
	"google.golang.org/grpc"
	gnoi_system_pb "github.com/openconfig/gnoi/system"
	spb "github.com/Azure/sonic-telemetry/proto/gnoi"
	"context"
	"os"
	"os/signal"
	"fmt"
	"flag"
	"google.golang.org/grpc/metadata"
	"github.com/google/gnxi/utils/credentials"
	"encoding/json"
)

var (
	module = flag.String("module", "System", "gNOI Module")
	rpc = flag.String("rpc", "Time", "rpc call in specified module to call")
	target = flag.String("target", "localhost:8080", "Address:port of gNOI Server")
	args = flag.String("jsonin", "", "RPC Arguments in json format")
	jwtToken = flag.String("jwt_token", "", "JWT Token if required")
	targetName = flag.String("target_name", "hostname.com", "The target name use to verify the hostname returned by TLS handshake")
)
func setUserCreds(ctx context.Context) context.Context {
	if len(*jwtToken) > 0 {
		ctx = metadata.AppendToOutgoingContext(ctx, "access_token", *jwtToken)
	}
	return ctx
}
func main() {
	flag.Parse()
	opts := credentials.ClientCredentials(*targetName)

    ctx, cancel := context.WithCancel(context.Background())
    go func() {
            c := make(chan os.Signal, 1)
            signal.Notify(c, os.Interrupt)
            <-c
            cancel()
    }()
	conn, err := grpc.Dial(*target, opts...)
	if err != nil {
		panic(err.Error())
	}
	
	switch *module {
	case "System":
		sc := gnoi_system_pb.NewSystemClient(conn)
		switch *rpc {
		case "Time":
			systemTime(sc, ctx)
		default:
			panic("Invalid RPC Name")
		}
	case "Sonic":
		sc := spb.NewSonicServiceClient(conn)
		switch *rpc {
		case "showtechsupport":
			sonicShowTechSupport(sc, ctx)
		case "sum":
			sonicSum(sc, ctx)
		case "copyConfig":
			copyConfig(sc, ctx)
		case "authenticate":
			authenticate(sc, ctx)
		case "imageInstall":
			imageInstall(sc, ctx)
		case "imageDefault":
			imageDefault(sc, ctx)
		case "imageRemove":
			imageRemove(sc, ctx)
		case "refresh":
			refresh(sc, ctx)
        case "clearNeighbors":
            clearNeighbors(sc, ctx)
		default:
			panic("Invalid RPC Name")
		}
	default:
		panic("Invalid Module Name")
	}

}

func systemTime(sc gnoi_system_pb.SystemClient, ctx context.Context) {
	fmt.Println("System Time")
	ctx = setUserCreds(ctx)
	resp,err := sc.Time(ctx, new(gnoi_system_pb.TimeRequest))
	if err != nil {
		panic(err.Error())
	}
	respstr, err := json.Marshal(resp)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(string(respstr))
}
func sonicShowTechSupport(sc spb.SonicServiceClient, ctx context.Context) {
	fmt.Println("Sonic ShowTechsupport")
	ctx = setUserCreds(ctx)
	req := &spb.TechsupportRequest {
		Input: &spb.TechsupportRequest_Input{
			
		},
	}

	json.Unmarshal([]byte(*args), req)
	
	resp,err := sc.ShowTechsupport(ctx, req)
	if err != nil {
		panic(err.Error())
	}
	respstr, err := json.Marshal(resp)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(string(respstr))
}
func sonicSum(sc spb.SonicServiceClient, ctx context.Context) {
	fmt.Println("Sonic Sum")
	ctx = setUserCreds(ctx)
	req := &spb.SumRequest{
		Input: &spb.SumRequest_Input{},
	}
	
	json.Unmarshal([]byte(*args), req)

	resp,err := sc.Sum(ctx, req)

	if err != nil {
		panic(err.Error())
	}
	respstr, err := json.Marshal(resp)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(string(respstr))
}

func copyConfig(sc spb.SonicServiceClient, ctx context.Context) {
	fmt.Println("Sonic CopyConfig")
	ctx = setUserCreds(ctx)
	req := &spb.CopyConfigRequest{
		Input: &spb.CopyConfigRequest_Input{},
	}
	json.Unmarshal([]byte(*args), req)

	resp,err := sc.CopyConfig(ctx, req)

	if err != nil {
		panic(err.Error())
	}
	respstr, err := json.Marshal(resp)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(string(respstr))
}

func imageInstall(sc spb.SonicServiceClient, ctx context.Context) {
	fmt.Println("Sonic ImageInstall")
	ctx = setUserCreds(ctx)
	req := &spb.ImageInstallRequest{
		Input: &spb.ImageInstallRequest_Input{},
	}
	json.Unmarshal([]byte(*args), req)

	resp,err := sc.ImageInstall(ctx, req)

	if err != nil {
		panic(err.Error())
	}
	respstr, err := json.Marshal(resp)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(string(respstr))
}
func imageRemove(sc spb.SonicServiceClient, ctx context.Context) {
	fmt.Println("Sonic ImageRemove")
	ctx = setUserCreds(ctx)
	req := &spb.ImageRemoveRequest{
		Input: &spb.ImageRemoveRequest_Input{},
	}
	json.Unmarshal([]byte(*args), req)

	resp,err := sc.ImageRemove(ctx, req)

	if err != nil {
		panic(err.Error())
	}
	respstr, err := json.Marshal(resp)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(string(respstr))
}

func imageDefault(sc spb.SonicServiceClient, ctx context.Context) {
	fmt.Println("Sonic ImageDefault")
	ctx = setUserCreds(ctx)
	req := &spb.ImageDefaultRequest{
		Input: &spb.ImageDefaultRequest_Input{},
	}
	json.Unmarshal([]byte(*args), req)

	resp,err := sc.ImageDefault(ctx, req)

	if err != nil {
		panic(err.Error())
	}
	respstr, err := json.Marshal(resp)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(string(respstr))
}

func authenticate(sc spb.SonicServiceClient, ctx context.Context) {
	fmt.Println("Sonic Authenticate")
	ctx = setUserCreds(ctx)
	req := &spb.AuthenticateRequest {}
	
	json.Unmarshal([]byte(*args), req)
	
	resp,err := sc.Authenticate(ctx, req)
	if err != nil {
		panic(err.Error())
	}
	respstr, err := json.Marshal(resp)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(string(respstr))
}

func refresh(sc spb.SonicServiceClient, ctx context.Context) {
	fmt.Println("Sonic Refresh")
	ctx = setUserCreds(ctx)
	req := &spb.RefreshRequest {}
	
	json.Unmarshal([]byte(*args), req)

	resp,err := sc.Refresh(ctx, req)
	if err != nil {
		panic(err.Error())
	}
	respstr, err := json.Marshal(resp)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(string(respstr))
}

func clearNeighbors(sc spb.SonicServiceClient, ctx context.Context) {
    fmt.Println("Sonic ClearNeighbors")
    ctx = setUserCreds(ctx)
    req := &spb.ClearNeighborsRequest{
        Input: &spb.ClearNeighborsRequest_Input{},
    }
    json.Unmarshal([]byte(*args), req)

    resp,err := sc.ClearNeighbors(ctx, req)

    if err != nil {
        panic(err.Error())
    }
    respstr, err := json.Marshal(resp)
    if err != nil {
        panic(err.Error())
    }
    fmt.Println(string(respstr))
}
