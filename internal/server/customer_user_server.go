package server

import (
	"context"
	"time"

	"github.com/sbasestarter/bizinters/userinters"
	anonymousauthenticator "github.com/sbasestarter/userlib/authenticator/anonymous"
	"github.com/sgostarter/i/commerr"
	"github.com/zservicer/protorepo/gens/talkpb"
	"github.com/zservicer/talkbe/internal/defs"
	"google.golang.org/grpc/codes"
)

func NewCustomerUserServer(user userinters.UserCenter, tokenHelper defs.UserTokenHelper) talkpb.CustomerUserServicerServer {
	return &customerUserServerImpl{
		user:        user,
		tokenHelper: tokenHelper,
	}
}

type customerUserServerImpl struct {
	talkpb.UnimplementedCustomerUserServicerServer

	user        userinters.UserCenter
	tokenHelper defs.UserTokenHelper
}

func (impl *customerUserServerImpl) CheckToken(ctx context.Context, request *talkpb.CheckTokenRequest) (*talkpb.CheckTokenResponse, error) {
	newToken, userName, err := impl.checkToken(ctx)
	if err != nil {
		return &talkpb.CheckTokenResponse{
			Valid: false,
		}, nil
	}

	return &talkpb.CheckTokenResponse{
		Valid:    true,
		UserName: userName,
		NewToken: newToken,
	}, nil
}

func (impl *customerUserServerImpl) checkToken(ctx context.Context) (newToken, userName string, err error) {
	newToken, _, userName, err = impl.tokenHelper.ExtractUserFromGRPCContext(ctx, true)

	return
}

func (impl *customerUserServerImpl) CreateToken(ctx context.Context, request *talkpb.CreateTokenRequest) (*talkpb.CreateTokenResponse, error) {
	if request == nil {
		return nil, gRPCMessageError(codes.InvalidArgument, "noRequest")
	}

	token, userName, err := impl.createToken(ctx, request.GetUserName())
	if err != nil {
		return nil, gRPCError(codes.Internal, err)
	}

	return &talkpb.CreateTokenResponse{
		Token:    token,
		UserName: userName,
	}, nil
}

func (impl *customerUserServerImpl) createToken(ctx context.Context, userName string) (token, tokenUserName string, err error) {
	tokenUserName = userName
	if tokenUserName == "" {
		tokenUserName = "Guest"
	}

	resp, err := impl.user.Login(ctx, &userinters.LoginRequest{
		ContinueID:        0,
		Authenticators:    []userinters.Authenticator{anonymousauthenticator.NewAuthenticator(tokenUserName)},
		TokenLiveDuration: time.Hour * 24 * 7,
	})
	if err != nil {
		return
	}

	if resp.Status != userinters.LoginStatusSuccess {
		err = commerr.ErrInternal

		return
	}

	token = resp.Token

	return
}
