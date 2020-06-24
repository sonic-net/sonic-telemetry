package gnmi

import (
	"context"
	gnoi_system_pb "github.com/openconfig/gnoi/system"
	log "github.com/golang/glog"
	"time"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
)

func (srv *Server) Reboot(ctx context.Context, req *gnoi_system_pb.RebootRequest) (*gnoi_system_pb.RebootResponse, error) {
	log.V(1).Info("gNOI: Reboot")
	return nil, status.Errorf(codes.Unimplemented, "")
}
func (srv *Server) RebootStatus(ctx context.Context, req *gnoi_system_pb.RebootStatusRequest) (*gnoi_system_pb.RebootStatusResponse, error) {
	log.V(1).Info("gNOI: RebootStatus")
	return nil, status.Errorf(codes.Unimplemented, "")
}
func (srv *Server) CancelReboot(ctx context.Context, req *gnoi_system_pb.CancelRebootRequest) (*gnoi_system_pb.CancelRebootResponse, error) {
	log.V(1).Info("gNOI: CancelReboot")
	return nil, status.Errorf(codes.Unimplemented, "")
}
func (srv *Server) Ping(req *gnoi_system_pb.PingRequest, rs gnoi_system_pb.System_PingServer) error {
	log.V(1).Info("gNOI: Ping")
	return status.Errorf(codes.Unimplemented, "")
}
func (srv *Server) Traceroute(req *gnoi_system_pb.TracerouteRequest, rs gnoi_system_pb.System_TracerouteServer) error {
	log.V(1).Info("gNOI: Traceroute")
	return status.Errorf(codes.Unimplemented, "")
}
func (srv *Server) SetPackage(rs gnoi_system_pb.System_SetPackageServer) error {
	log.V(1).Info("gNOI: SetPackage")
	return status.Errorf(codes.Unimplemented, "")
}
func (srv *Server) SwitchControlProcessor(ctx context.Context, req *gnoi_system_pb.SwitchControlProcessorRequest) (*gnoi_system_pb.SwitchControlProcessorResponse, error) {
	log.V(1).Info("gNOI: SwitchControlProcessor")
	return nil, status.Errorf(codes.Unimplemented, "")
}
func (srv *Server) Time(ctx context.Context, req *gnoi_system_pb.TimeRequest) (*gnoi_system_pb.TimeResponse, error) {
	log.V(1).Info("gNOI: Time")
	var tm gnoi_system_pb.TimeResponse
	tm.Time = uint64(time.Now().UnixNano())
	return &tm, nil
}

