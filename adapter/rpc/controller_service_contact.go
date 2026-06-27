package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
)

// ListContacts implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) ListContacts(ctx context.Context, req *connect.Request[hdlctrlv1.ListContactsRequest]) (*connect.Response[hdlctrlv1.ListContactsResponse], error) {
	hosts, err := c.hhrepo.ListRunningByAccount(ctx, req.Msg.GetHeadlessAccountId())
	if err != nil {
		return nil, convertErr(err)
	}

	if len(hosts) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("このアカウントで起動中のヘッドレスホストが必要です"))
	}

	conn, err := c.hhrepo.GetRpcClient(ctx, hosts[0].ID)
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	headlessRes, err := conn.ListContacts(ctx, &headlessv1.ListContactsRequest{
		Limit:  req.Msg.GetLimit(),
		Cursor: req.Msg.Cursor,
	})
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	contacts := make([]*hdlctrlv1.UserInfo, 0, len(headlessRes.GetUsers()))
	for _, u := range headlessRes.GetUsers() {
		contacts = append(contacts, &hdlctrlv1.UserInfo{
			Id:      u.GetId(),
			Name:    u.GetName(),
			IconUrl: u.GetIconUrl(),
		})
	}

	return connect.NewResponse(&hdlctrlv1.ListContactsResponse{
		Contacts:   contacts,
		NextCursor: headlessRes.NextCursor,
	}), nil
}

// GetContactMessages implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) GetContactMessages(ctx context.Context, req *connect.Request[hdlctrlv1.GetContactMessagesRequest]) (*connect.Response[hdlctrlv1.GetContactMessagesResponse], error) {
	account, err := c.hauc.GetHeadlessAccount(ctx, req.Msg.GetHeadlessAccountId())
	if err != nil {
		return nil, convertErr(err)
	}

	hosts, err := c.hhrepo.ListRunningByAccount(ctx, req.Msg.GetHeadlessAccountId())
	if err != nil {
		return nil, convertErr(err)
	}

	if len(hosts) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("このアカウントで起動中のヘッドレスホストが必要です"))
	}

	conn, err := c.hhrepo.GetRpcClient(ctx, hosts[0].ID)
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	headlessRes, err := conn.GetContactMessages(ctx, &headlessv1.GetContactMessagesRequest{
		UserId:   req.Msg.GetContactUserId(),
		Limit:    req.Msg.GetLimit(),
		BeforeId: req.Msg.BeforeId,
		AfterId:  req.Msg.AfterId,
	})
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	messages := make([]*hdlctrlv1.ContactMessage, 0, len(headlessRes.GetMessages()))
	for _, m := range headlessRes.GetMessages() {
		messages = append(messages, &hdlctrlv1.ContactMessage{
			Id:           m.GetId(),
			Type:         m.GetType(),
			Content:      m.GetContent(),
			SendTime:     m.GetSendTime(),
			ReadTime:     m.GetReadTime(),
			IsOwnMessage: m.GetSenderId() == account.ResoniteID,
		})
	}

	return connect.NewResponse(&hdlctrlv1.GetContactMessagesResponse{
		Messages:      messages,
		HasMoreBefore: headlessRes.GetHasMoreBefore(),
		HasMoreAfter:  headlessRes.GetHasMoreAfter(),
	}), nil
}

// SendContactMessage implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) SendContactMessage(ctx context.Context, req *connect.Request[hdlctrlv1.SendContactMessageRequest]) (*connect.Response[hdlctrlv1.SendContactMessageResponse], error) {
	hosts, err := c.hhrepo.ListRunningByAccount(ctx, req.Msg.GetHeadlessAccountId())
	if err != nil {
		return nil, convertErr(err)
	}

	if len(hosts) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("このアカウントで起動中のヘッドレスホストが必要です"))
	}

	conn, err := c.hhrepo.GetRpcClient(ctx, hosts[0].ID)
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	_, err = conn.SendContactMessage(ctx, &headlessv1.SendContactMessageRequest{
		UserId:  req.Msg.GetContactUserId(),
		Message: req.Msg.GetMessage(),
	})
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.SendContactMessageResponse{}), nil
}
