package rpc

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/converter"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var _ hdlctrlv1connect.ControllerServiceHandler = (*ControllerService)(nil)

type ControllerService struct {
	connections map[string]headlessv1.HeadlessControlServiceClient
	hhrepo      port.HeadlessHostRepository
}

func NewControllerService(hhrepo port.HeadlessHostRepository) *ControllerService {
	return &ControllerService{
		connections: make(map[string]headlessv1.HeadlessControlServiceClient),
		hhrepo:      hhrepo,
	}
}

func (c *ControllerService) Handle(mux *http.ServeMux) {
	interceptors := connect.WithInterceptors(auth.NewAuthInterceptor())
	path, handler := hdlctrlv1connect.NewControllerServiceHandler(c, interceptors)
	mux.Handle(path, handler)
}

// GetHeadlessHost implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) GetHeadlessHost(ctx context.Context, req *connect.Request[hdlctrlv1.GetHeadlessHostRequest]) (*connect.Response[hdlctrlv1.GetHeadlessHostResponse], error) {
	host, err := c.hhrepo.Find(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if host == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("host not found"))
	}
	res := connect.NewResponse(&hdlctrlv1.GetHeadlessHostResponse{
		Host: converter.HeadlessHostEntityToProto(host),
	})
	return res, nil
}

// ListHeadlessHost implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) ListHeadlessHost(ctx context.Context, req *connect.Request[hdlctrlv1.ListHeadlessHostRequest]) (*connect.Response[hdlctrlv1.ListHeadlessHostResponse], error) {
	hosts, err := c.hhrepo.ListAll()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	protoHosts := make([]*hdlctrlv1.HeadlessHost, 0, len(hosts))
	for _, host := range hosts {
		protoHosts = append(protoHosts, converter.HeadlessHostEntityToProto(host))
	}
	res := connect.NewResponse(&hdlctrlv1.ListHeadlessHostResponse{
		Hosts: protoHosts,
	})
	return res, nil
}

// StartWorld implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) StartWorld(ctx context.Context, req *connect.Request[hdlctrlv1.StartWorldRequest]) (*connect.Response[hdlctrlv1.StartWorldResponse], error) {
	conn, err := c.getOrNewConnection(req.Msg.HostId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	headlessRes, err := conn.StartWorld(ctx, &headlessv1.StartWorldRequest{Parameters: req.Msg.Parameters})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&hdlctrlv1.StartWorldResponse{
		OpenedSession: headlessRes.OpenedSession,
	})
	return res, nil
}

// InviteUser implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) InviteUser(ctx context.Context, req *connect.Request[hdlctrlv1.InviteUserRequest]) (*connect.Response[hdlctrlv1.InviteUserResponse], error) {
	conn, err := c.getOrNewConnection(req.Msg.HostId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	hreq := &headlessv1.InviteUserRequest{
		SessionId: req.Msg.SessionId,
	}
	if req.Msg.GetUserId() != "" {
		hreq.User = &headlessv1.InviteUserRequest_UserId{UserId: req.Msg.GetUserId()}
	} else {
		hreq.User = &headlessv1.InviteUserRequest_UserName{UserName: req.Msg.GetUserName()}
	}
	_, err = conn.InviteUser(ctx, hreq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&hdlctrlv1.InviteUserResponse{})
	return res, nil
}

// StopSession implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) StopSession(ctx context.Context, req *connect.Request[hdlctrlv1.StopSessionRequest]) (*connect.Response[hdlctrlv1.StopSessionResponse], error) {
	conn, err := c.getOrNewConnection(req.Msg.HostId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	_, err = conn.StopSession(ctx, &headlessv1.StopSessionRequest{SessionId: req.Msg.SessionId})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&hdlctrlv1.StopSessionResponse{})
	return res, nil
}

func (c *ControllerService) getOrNewConnection(id string) (headlessv1.HeadlessControlServiceClient, error) {
	if conn, ok := c.connections[id]; ok {
		return conn, nil
	}

	h, err := c.hhrepo.Find(id)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(h.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	client := headlessv1.NewHeadlessControlServiceClient(conn)
	c.connections[id] = client

	return client, nil
}
