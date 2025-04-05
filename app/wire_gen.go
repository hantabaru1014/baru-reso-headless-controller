// Code generated by Wire. DO NOT EDIT.

//go:generate go run -mod=mod github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package app

import (
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/rpc"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/worker"
)

import (
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
)

// Injectors from wire.go:

func InitializeServer() *Server {
	queries := db.NewQueries()
	userUsecase := usecase.NewUserUsecase(queries)
	userService := rpc.NewUserService(userUsecase)
	headlessHostRepository := adapter.NewHeadlessHostRepository()
	headlessHostUsecase := usecase.NewHeadlessHostUsecase(headlessHostRepository)
	headlessAccountUsecase := usecase.NewHeadlessAccountUsecase(queries)
	controllerService := rpc.NewControllerService(headlessHostRepository, headlessHostUsecase, headlessAccountUsecase)
	imageChecker := worker.NewImageChecker(headlessHostRepository)
	server := NewServer(userService, controllerService, imageChecker)
	return server
}

func InitializeCli() *Cli {
	queries := db.NewQueries()
	userUsecase := usecase.NewUserUsecase(queries)
	cli := NewCli(userUsecase)
	return cli
}
