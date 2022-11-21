package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/sbasestarter/bizinters/talkinters"
	"github.com/sbasestarter/bizinters/userinters"
	"github.com/sbasestarter/bizinters/userinters/userpass"
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

// nolint: funlen
func main() {
	cfg := config.GetConfig()

	logger := cfg.Logger

	tlsConfig, err := servicetoolset.GRPCTlsConfigMap(cfg.GRPCTLSConfig)
	if err != nil {
		logger.Fatal(err)
	}

	grpcCfg := &servicetoolset.GRPCServerConfig{
		Address:           cfg.Listen,
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

	var rM talkinters.Model
	if cfg.Dev.UseMemoryModel {
		rM = impls.NewMemModel()
	} else {
		rM, err = model.NewMongoModel(cfg.TalkMongoDSN, logger)
		if err != nil {
			logger.Fatal(err)
		}
	}

	modelEx := impls.NewModelEx(rM)
	mdi := impls.NewAllInOneMDI(modelEx, logger)

	customerUserCenter := userlib.NewUserCenter(cfg.CustomerTokenSecret, single.NewPolicy(userinters.AuthMethodNameAnonymous),
		memorystatuscontroller.NewStatusController(), memoryauthingdatastorage.NewMemoryAuthingDataStorage(), logger)
	customerUserTokenHelper := impls.NewLocalCustomerUserTokenHelper(customerUserCenter)
	customerMD := impls.NewCustomerMD(mdi, logger)
	customerController := controller.NewCustomerController(customerMD, modelEx, logger)
	grpcCustomerServer := server.NewCustomerServer(customerController, customerUserTokenHelper, modelEx, logger)
	grpcCustomerUserServer := server.NewCustomerUserServer(customerUserCenter, customerUserTokenHelper)

	var serviceUserPassModel userpass.UserPasswordModel

	if cfg.Dev.UseMemoryModel {
		serviceUserPassModel = impls.NewMemUserPassModel()
	} else {
		mongoCli, mongoOptions, errMongo := mongolib.InitMongo(cfg.UserMongoDSN)
		if errMongo != nil {
			logger.Fatal(errMongo)

			return
		}
		serviceUserPassModel = userpassauthenticator.NewMongoUserPasswordModel(mongoCli, mongoOptions.Auth.AuthSource, "servicer_users", logger)
	}

	servicerUserCenter := userlib.NewUserCenter(cfg.ServicerTokenSecret, single.NewPolicy(userinters.AuthMethodNameUserPassword),
		memorystatuscontroller.NewStatusController(), memoryauthingdatastorage.NewMemoryAuthingDataStorage(), logger)

	servicerManager := userpassmanager.NewManager(cfg.ServicerPasswordSecret, serviceUserPassModel)
	userID, err := servicerManager.Register(context.TODO(), "demo", "123456")

	if err == nil {
		_ = servicerManager.UpdateUserAllExData(context.TODO(), userID, map[string]interface{}{
			"permission": 1,
			"actIDs": []string{
				"actIDDemo",
			},
			"bizIDs": []string{
				"bizIDDemo",
			},
		})
	}

	servicerUserTokenHelper := impls.NewLocalServicerUserTokenHelper(servicerUserCenter, servicerManager)

	servicerMD := impls.NewServicerMD(mdi, logger)
	servicerController := controller.NewServicerController(servicerMD, modelEx, logger)
	grpcServicerServer := server.NewServicerServer(servicerController, servicerUserTokenHelper, modelEx, logger)
	grpcServicerUserServer := server.NewServicerUserServer(servicerManager, servicerUserCenter, servicerUserTokenHelper)

	err = s.Start(func(s *grpc.Server) error {
		talkpb.RegisterCustomerTalkServiceServer(s, grpcCustomerServer)
		talkpb.RegisterServiceTalkServiceServer(s, grpcServicerServer)
		talkpb.RegisterCustomerUserServicerServer(s, grpcCustomerUserServer)
		talkpb.RegisterServicerUserServicerServer(s, grpcServicerUserServer)

		return nil
	})
	if err != nil {
		logger.Fatal(err)

		return
	}

	go func() {
		time.Sleep(time.Second)

		wsCfg := config.GetWSConfig()
		s := &http.Server{
			Addr:              wsCfg.CustomerListen,
			ReadHeaderTimeout: 3 * time.Second,
			Handler:           http.NewServeMux(),
		}

		mux := http.NewServeMux()
		s.Handler = mux

		server.SetupHTTPCustomerServer(mux, wsCfg)

		logger.Info("customer ws server listen on: ", wsCfg.CustomerListen)

		log.Fatal(s.ListenAndServe())
	}()

	go func() {
		time.Sleep(time.Second)

		wsCfg := config.GetWSConfig()
		s := &http.Server{
			Addr:              wsCfg.ServicerListen,
			ReadHeaderTimeout: 3 * time.Second,
			Handler:           http.NewServeMux(),
		}

		mux := http.NewServeMux()
		s.Handler = mux

		server.SetupHTTPServicerServer(mux, wsCfg)

		logger.Info("servicer ws server listen on: ", wsCfg.ServicerListen)

		log.Fatal(s.ListenAndServe())
	}()

	logger.Info("grpc server listen on: ", cfg.Listen)
	s.Wait()
}
