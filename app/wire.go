//go:build wireinject
// +build wireinject

package app

import (
	"github.com/google/wire"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/rpc"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

var (
	repositorySet = wire.NewSet(
		wire.Bind(new(port.HeadlessHostRepository), new(*adapter.HeadlessHostRepository)),
		adapter.NewHeadlessHostRepository,
	)
	rpcServiceSet = wire.NewSet(
		rpc.NewUserService,
		rpc.NewControllerService,
	)
)

func InitializeServer() *Server {
	wire.Build(
		repositorySet,
		rpcServiceSet,
		NewServer,
	)
	return &Server{}
}
