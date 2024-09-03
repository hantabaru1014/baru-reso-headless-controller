//go:build wireinject
// +build wireinject

package app

import (
	"github.com/google/wire"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/rpc"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

var (
	repositorySet = wire.NewSet(
		wire.Bind(new(port.HeadlessHostRepository), new(*db.HeadlessHostRepository)),
		db.NewHeadlessHostRepository,
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
