package main

import (
	"time"

	"github.com/sbasestarter/bizinters/userinters"
	"github.com/sbasestarter/bizmongolib/talk/model"
	"github.com/sbasestarter/userlib"
	memoryauthingdatastorage "github.com/sbasestarter/userlib/authingdatastorage/memory"
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
		Address:           cfg.CustomerListen,
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

	customerUserCenter := userlib.NewUserCenter(cfg.CustomerTokenSecret, single.NewPolicy(userinters.AuthMethodNameAnonymous),
		memorystatuscontroller.NewStatusController(), memoryauthingdatastorage.NewMemoryAuthingDataStorage(), logger)
	customerUserTokenHelper := impls.NewLocalCustomerUserTokenHelper(customerUserCenter)

	rM, err := model.NewMongoModel(cfg.TalkMongoDSN, logger)
	if err != nil {
		logger.Fatal(err)

		return
	}

	modelEx := impls.NewModelEx(rM)
	mdi := impls.NewCustomerRabbitMQMDI(cfg.RabbitMQURL, modelEx, logger)

	customerMD := impls.NewCustomerMD(mdi, logger)

	customerController := controller.NewCustomerController(customerMD, modelEx, logger)

	grpcCustomerServer := server.NewCustomerServer(customerController, customerUserTokenHelper, modelEx, logger)

	err = s.Start(func(s *grpc.Server) error {
		talkpb.RegisterCustomerTalkServiceServer(s, grpcCustomerServer)

		return nil
	})
	if err != nil {
		logger.Fatal(err)

		return
	}

	logger.Info("grpc server listen on: ", cfg.CustomerListen)
	s.Wait()
}
