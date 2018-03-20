package gnmi

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	sdc "github.com/Azure/sonic-telemetry/sonic_data_client"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

var (
	supportedEncodings = []gnmipb.Encoding{gnmipb.Encoding_JSON, gnmipb.Encoding_JSON_IETF}
)

// Server manages a single gNMI Server implementation. Each client that connects
// via Subscribe or Get will receive a stream of updates based on the requested
// path. Set request is processed by server too.
type Server struct {
	s       *grpc.Server
	lis     net.Listener
	config  *Config
	cMu     sync.Mutex
	clients map[string]*Client
}

// Config is a collection of values for Server
type Config struct {
	// Port for the Server to listen on. If 0 or unset the Server will pick a port
	// for this Server.
	Port int64
}

// New returns an initialized Server.
func NewServer(config *Config, opts []grpc.ServerOption) (*Server, error) {
	if config == nil {
		return nil, errors.New("config not provided")
	}

	s := grpc.NewServer(opts...)
	reflection.Register(s)

	srv := &Server{
		s:       s,
		config:  config,
		clients: map[string]*Client{},
	}
	var err error
	if srv.config.Port < 0 {
		srv.config.Port = 0
	}
	srv.lis, err = net.Listen("tcp", fmt.Sprintf(":%d", srv.config.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to open listener port %d: %v", srv.config.Port, err)
	}
	gnmipb.RegisterGNMIServer(srv.s, srv)
	log.V(1).Infof("Created Server on %s", srv.Address())
	return srv, nil
}

// Serve will start the Server serving and block until closed.
func (srv *Server) Serve() error {
	s := srv.s
	if s == nil {
		return fmt.Errorf("Serve() failed: not initialized")
	}
	return srv.s.Serve(srv.lis)
}

// Address returns the port the Server is listening to.
func (srv *Server) Address() string {
	addr := srv.lis.Addr().String()
	return strings.Replace(addr, "[::]", "localhost", 1)
}

// Port returns the port the Server is listening to.
func (srv *Server) Port() int64 {
	return srv.config.Port
}

// Subscribe implements the gNMI Subscribe RPC.
func (srv *Server) Subscribe(stream gnmipb.GNMI_SubscribeServer) error {
	ctx := stream.Context()

	pr, ok := peer.FromContext(ctx)
	if !ok {
		return grpc.Errorf(codes.InvalidArgument, "failed to get peer from ctx")
		//return fmt.Errorf("failed to get peer from ctx")
	}
	if pr.Addr == net.Addr(nil) {
		return grpc.Errorf(codes.InvalidArgument, "failed to get peer address")
	}

	/* TODO: authorize the user
	msg, ok := credentials.AuthorizeUser(ctx)
	if !ok {
		log.Infof("denied a Set request: %v", msg)
		return nil, status.Error(codes.PermissionDenied, msg)
	}
	*/

	c := NewClient(pr.Addr)

	srv.cMu.Lock()
	if oc, ok := srv.clients[c.String()]; ok {
		log.V(2).Infof("Delete duplicate client %s", oc)
		oc.Close()
		delete(srv.clients, c.String())
	}
	srv.clients[c.String()] = c
	srv.cMu.Unlock()

	err := c.Run(stream)
	srv.cMu.Lock()
	delete(srv.clients, c.String())
	srv.cMu.Unlock()

	log.Flush()
	return err
}

// checkEncodingAndModel checks whether encoding and models are supported by the server. Return error if anything is unsupported.
func (s *Server) checkEncodingAndModel(encoding gnmipb.Encoding, models []*gnmipb.ModelData) error {
	hasSupportedEncoding := false
	for _, supportedEncoding := range supportedEncodings {
		if encoding == supportedEncoding {
			hasSupportedEncoding = true
			break
		}
	}
	if !hasSupportedEncoding {
		return fmt.Errorf("unsupported encoding: %s", gnmipb.Encoding_name[int32(encoding)])
	}

	return nil
}

// Get implements the Get RPC in gNMI spec.
func (s *Server) Get(ctx context.Context, req *gnmipb.GetRequest) (*gnmipb.GetResponse, error) {
	var err error

	if req.GetType() != gnmipb.GetRequest_ALL {
		return nil, status.Errorf(codes.Unimplemented, "unsupported request type: %s", gnmipb.GetRequest_DataType_name[int32(req.GetType())])
	}

	if err = s.checkEncodingAndModel(req.GetEncoding(), req.GetUseModels()); err != nil {
		return nil, status.Error(codes.Unimplemented, err.Error())
	}

	var target string
	prefix := req.GetPrefix()
	if prefix == nil {
		return nil, status.Error(codes.Unimplemented, "No target specified in prefix")
	} else {
		target = prefix.GetTarget()
		if target == "" {
			return nil, status.Error(codes.Unimplemented, "Empty target data not supported yet")
		}
	}

	paths := req.GetPath()
	log.V(5).Infof("GetRequest paths: %v", paths)

	var dc sdc.Client
	if target == "OTHERS" {
		dc, err = sdc.NewNonDbClient(paths, prefix)
	} else {
		dc, err = sdc.NewDbClient(paths, prefix)
	}
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	notifications := make([]*gnmipb.Notification, len(paths))
	spbValues, err := dc.Get(nil)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	for index, spbValue := range spbValues {
		update := &gnmipb.Update{
			Path: spbValue.GetPath(),
			Val:  spbValue.GetVal(),
		}

		notifications[index] = &gnmipb.Notification{
			Timestamp: spbValue.GetTimestamp(),
			Prefix:    prefix,
			Update:    []*gnmipb.Update{update},
		}
		index++
	}
	return &gnmipb.GetResponse{Notification: notifications}, nil
}

// Get string value from TypedValue in gnmipb
func getUpdateVal(typedVal *gnmipb.TypedValue) (string, error) {
	intVal := typedVal.GetIntVal()
	stringVal := typedVal.GetStringVal()
	bytesVal := typedVal.GetBytesVal()
	if stringVal != "" {
		return stringVal, nil
	}
	if intVal != 0 {
		return strconv.FormatInt(intVal, 10), nil
	}
	if bytesVal != nil {
		return string(bytesVal[:]), nil
	}
	return "", fmt.Errorf("typedVal: %v not supported", typedVal)
}

// Set implements the Get RPC in gNMI spec.
func (srv *Server) Set(ctx context.Context, req *gnmipb.SetRequest) (*gnmipb.SetResponse, error) {
	var target string
	prefix := req.GetPrefix()
	if prefix == nil {
		return nil, status.Error(codes.Unimplemented, "No target specified in prefix")
	}

	target = prefix.GetTarget()
	if target == "" {
		return nil, status.Error(codes.Unimplemented, "Empty target data not supported yet")
	}

	// only support set config_db
	if target != "CONFIG_DB" {
		return nil, status.Errorf(codes.Unimplemented, "unsupported request target")
	}

	var results []*gnmipb.UpdateResult
	dc, err := sdc.NewDbClient(nil, prefix)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	for _, path := range req.GetDelete() {
		log.V(5).Infof("Delete path: %v", path)
		err := dc.Set(path, "")
		if err != nil {
			return nil, err
		}
		res := gnmipb.UpdateResult{
			Path: path,
			Op:   gnmipb.UpdateResult_DELETE,
		}
		results = append(results, &res)
	}

	for _, path := range req.GetReplace() {
		valString, err := getUpdateVal(path.GetVal())
		if err != nil {
			return nil, err
		}
		log.V(5).Infof("Replace path: %v valString: %v", path, valString)
		err = dc.Set(path.GetPath(), valString)
		if err != nil {
			return nil, err
		}
		res := gnmipb.UpdateResult{
			Path: path.GetPath(),
			Op:   gnmipb.UpdateResult_REPLACE,
		}
		results = append(results, &res)
	}

	for _, path := range req.GetUpdate() {
		valString, err := getUpdateVal(path.GetVal())
		if err != nil {
			return nil, err
		}
		log.V(5).Infof("Update path: %v valString: %v", path, valString)
		err = dc.Set(path.GetPath(), valString)
		if err != nil {
			return nil, err
		}
		res := gnmipb.UpdateResult{
			Path: path.GetPath(),
			Op:   gnmipb.UpdateResult_UPDATE,
		}
		results = append(results, &res)
	}

	return &gnmipb.SetResponse{
		Timestamp: time.Now().UnixNano(),
		Prefix:    req.GetPrefix(),
		Response:  results,
	}, nil
}

// Capabilities method is not implemented. Refer to gnxi for examples with openconfig integration
func (srv *Server) Capabilities(context.Context, *gnmipb.CapabilityRequest) (*gnmipb.CapabilityResponse, error) {
	return nil, grpc.Errorf(codes.Unimplemented, "Capabilities() is not implemented")
}
