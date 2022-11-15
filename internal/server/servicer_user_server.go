package server

import (
	"context"
	"time"

	"github.com/sbasestarter/bizinters/userinters"
	"github.com/sbasestarter/userlib/authenticator/userpass"
	userpassmanager "github.com/sbasestarter/userlib/manager/userpass"
	"github.com/zservicer/protorepo/gens/talkpb"
	"github.com/zservicer/talkbe/internal/defs"
	"google.golang.org/grpc/codes"
)

func NewServicerUserServer(userManager userpassmanager.Manager, user userinters.UserCenter, tokenHelper defs.UserTokenHelper) talkpb.ServicerUserServicerServer {
	return &servicerUserServerImpl{
		userManager: userManager,
		user:        user,
		tokenHelper: tokenHelper,
	}
}

type servicerUserServerImpl struct {
	talkpb.UnimplementedServicerUserServicerServer

	userManager userpassmanager.Manager
	user        userinters.UserCenter
	tokenHelper defs.UserTokenHelper
}

func (impl *servicerUserServerImpl) Register(ctx context.Context, request *talkpb.RegisterRequest) (*talkpb.RegisterResponse, error) {
	token, code, err := impl.register(ctx, request)
	if code != codes.OK {
		return nil, gRPCError(code, err)
	}

	return &talkpb.RegisterResponse{
		Token:    token,
		UserName: request.GetUserName(),
	}, nil
}

func (impl *servicerUserServerImpl) Login(ctx context.Context, request *talkpb.LoginRequest) (*talkpb.LoginResponse, error) {
	if request == nil || request.GetUserName() == "" || request.GetPassword() == "" {
		return nil, gRPCMessageError(codes.InvalidArgument, "")
	}

	token, code, err := impl.login(ctx, request.GetUserName(), request.GetPassword())
	if code != codes.OK {
		return nil, gRPCError(code, err)
	}

	return &talkpb.LoginResponse{
		Token:    token,
		UserName: request.GetUserName(),
	}, nil
}

//
//
//

func (impl *servicerUserServerImpl) register(ctx context.Context, request *talkpb.RegisterRequest) (
	token string, code codes.Code, err error) {
	if request == nil || request.GetUserName() == "" || request.GetPassword() == "" {
		code = codes.InvalidArgument

		return
	}

	_, err = impl.userManager.Register(ctx, request.GetUserName(), request.GetPassword())
	if err != nil {
		code = codes.Internal

		return
	}

	token, code, err = impl.login(ctx, request.GetUserName(), request.GetPassword())
	if code != codes.OK {
		return
	}

	return
}

func (impl *servicerUserServerImpl) login(ctx context.Context, userName, password string) (
	token string, code codes.Code, err error) {
	authenticator, err := userpass.NewAuthenticator(userName, password, impl.userManager)
	if err != nil {
		code = codes.Internal

		return
	}

	resp, err := impl.user.Login(ctx, &userinters.LoginRequest{
		ContinueID: 0,
		Authenticators: []userinters.Authenticator{
			authenticator,
		},
		TokenLiveDuration: time.Hour * 24 * 7,
	})
	if err != nil {
		code = codes.Internal

		return
	}

	if resp.Status != userinters.LoginStatusSuccess {
		code = codes.Unauthenticated

		return
	}

	token = resp.Token
	code = codes.OK

	return
}
