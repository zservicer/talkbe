package main

import (
	"time"

	"github.com/sbasestarter/bizinters/userinters"
	"github.com/sbasestarter/bizmongolib/mongolib"
	"github.com/sbasestarter/bizmongolib/talk/model"
	userpassauthenticator "github.com/sbasestarter/bizmongolib/user/authenticator/userpass"
	"github.com/sbasestarter/userlib"
	memoryauthingdatastorage "github.com/sbasestarter/userlib/authingdatastorage/memory"
	userpassmanager "github.com/sbasestarter/userlib/manager/userpass"
	"github.com/sbasestarter/userlib/policy/single"
	memorystatuscontroller "github.com/sbasestarter/userlib/statuscontroller/memory"
	"github.com/sgostarter/libservicetoolset/servicetoolset"
	"github.com/zservicer/protorepo/gens/talkpb"
	"github.com/zservicer/talkbe/config"
	"github.com/zservicer/talkbe/internal/controller"
	"github.com/zservicer/talkbe/internal/impls"
	"github.com/zservicer/talkbe/internal/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func main() {
	cfg := config.GetConfig()

	logger := cfg.Logger

	tlsConfig, err := servicetoolset.GRPCTlsConfigMap(cfg.GRPCTLSConfig)
	if err != nil {
		logger.Fatal(err)
	}

	grpcCfg := &servicetoolset.GRPCServerConfig{
		Address:           cfg.ServicerListen,
		TLSConfig:         tlsConfig,
		KeepAliveDuration: time.Minute * 10,
	}

	s, err := servicetoolset.NewGRPCServer(nil, grpcCfg,
		[]grpc.ServerOption{grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             time.Second * 10,
			PermitWithoutStream: true,
		})}, nil, logger)
	if err != nil {
		logger.Fatal(err)

		return
	}

	mongoCli, mongoOptions, err := mongolib.InitMongo(cfg.UserMongoDSN)
	if err != nil {
		logger.Fatal(err)

		return
	}

	servicerUserCenter := userlib.NewUserCenter(cfg.ServicerTokenSecret, single.NewPolicy(userinters.AuthMethodNameUserPassword),
		memorystatuscontroller.NewStatusController(), memoryauthingdatastorage.NewMemoryAuthingDataStorage(), logger)
	serviceUserPassModel := userpassauthenticator.NewMongoUserPasswordModel(mongoCli, mongoOptions.Auth.AuthSource, "servicer_users", logger)
	servicerManager := userpassmanager.NewManager(cfg.ServicerPasswordSecret, serviceUserPassModel)
	servicerUserTokenHelper := impls.NewLocalServicerUserTokenHelper(servicerUserCenter, servicerManager)

	rM, err := model.NewMongoModel(cfg.TalkMongoDSN, logger)
	if err != nil {
		logger.Fatal(err)

		return
	}

	modelEx := impls.NewModelEx(rM)
	mdi := impls.NewServicerRabbitMQMDI(cfg.RabbitMQURL, modelEx, logger)

	servicerMD := impls.NewServicerMD(mdi, logger)

	servicerController := controller.NewServicerController(servicerMD, modelEx, logger)

	grpcServicerServer := server.NewServicerServer(servicerController, servicerUserTokenHelper, modelEx, logger)

	err = s.Start(func(s *grpc.Server) error {
		talkpb.RegisterServiceTalkServiceServer(s, grpcServicerServer)

		return nil
	})
	if err != nil {
		logger.Fatal(err)

		return
	}

	logger.Info("grpc server listen on: ", cfg.ServicerListen)
	s.Wait()
}
