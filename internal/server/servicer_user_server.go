package server

import (
	"context"
	"time"

	"github.com/sbasestarter/bizinters/userinters"
	"github.com/sbasestarter/userlib/authenticator/userpass"
	userpassmanager "github.com/sbasestarter/userlib/manager/userpass"
	"github.com/sgostarter/libeasygo/crypt/simencrypt"
	"github.com/zservicer/protorepo/gens/talkpb"
	"github.com/zservicer/talkbe/internal/defs"
	"google.golang.org/grpc/codes"
)

func NewServicerUserServer(userManager userpassmanager.Manager, user userinters.UserCenter, tokenHelper defs.ServicerUserTokenHelper) talkpb.ServicerUserServicerServer {
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
	tokenHelper defs.ServicerUserTokenHelper
}

func (impl *servicerUserServerImpl) Register(ctx context.Context, request *talkpb.RegisterRequest) (*talkpb.RegisterResponse, error) {
	userID, token, code, err := impl.register(ctx, request)
	if code != codes.OK {
		return nil, gRPCError(code, err)
	}

	return &talkpb.RegisterResponse{
		Token:    token,
		UserId:   simencrypt.EncryptUInt64(userID),
		UserName: request.GetUserName(),
	}, nil
}

func (impl *servicerUserServerImpl) Login(ctx context.Context, request *talkpb.LoginRequest) (*talkpb.LoginResponse, error) {
	if request == nil || request.GetUserName() == "" || request.GetPassword() == "" {
		return nil, gRPCMessageError(codes.InvalidArgument, "")
	}

	userID, token, code, err := impl.login(ctx, request.GetUserName(), request.GetPassword())
	if code != codes.OK {
		return nil, gRPCError(code, err)
	}

	// nolint: dogsled
	_, _, _, _, actIDs, bizIDs, _ := impl.tokenHelper.ExplainToken(ctx, token, false)

	return &talkpb.LoginResponse{
		Token:    token,
		UserId:   simencrypt.EncryptUInt64(userID),
		UserName: request.GetUserName(),
		ActIds:   actIDs,
		BizIds:   bizIDs,
	}, nil
}

//
//
//

func (impl *servicerUserServerImpl) register(ctx context.Context, request *talkpb.RegisterRequest) (
	userID uint64, token string, code codes.Code, err error) {
	if request == nil || request.GetUserName() == "" || request.GetPassword() == "" {
		code = codes.InvalidArgument

		return
	}

	_, err = impl.userManager.Register(ctx, request.GetUserName(), request.GetPassword())
	if err != nil {
		code = codes.Internal

		return
	}

	userID, token, code, err = impl.login(ctx, request.GetUserName(), request.GetPassword())
	if code != codes.OK {
		return
	}

	return
}

func (impl *servicerUserServerImpl) login(ctx context.Context, userName, password string) (
	userID uint64, token string, code codes.Code, err error) {
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

	userID = resp.UserID
	token = resp.Token
	code = codes.OK

	return
}

func (impl *servicerUserServerImpl) Profile(ctx context.Context, request *talkpb.ProfileRequest) (*talkpb.ProfileResponse, error) {
	resp, code, err := impl.profile(ctx, request)
	if code != codes.OK {
		return nil, gRPCError(code, err)
	}

	return resp, nil
}

func (impl *servicerUserServerImpl) profile(ctx context.Context, request *talkpb.ProfileRequest) (resp *talkpb.ProfileResponse, code codes.Code, err error) {
	code = codes.Unknown

	if request == nil {
		code = codes.InvalidArgument

		return
	}

	err = request.Validate()
	if err != nil {
		code = codes.InvalidArgument

		return
	}

	newToken, userID, userName, _, actIDs, bizIDs, err := impl.tokenHelper.ExtractUserFromGRPCContext(ctx, request.GetRenewToken())
	if err != nil {
		code = codes.Unauthenticated

		return
	}

	resp = &talkpb.ProfileResponse{
		Token:    newToken,
		UserId:   simencrypt.EncryptUInt64(userID),
		UserName: userName,
		ActIds:   actIDs,
		BizIds:   bizIDs,
	}

	code = codes.OK

	return
}

func (impl *servicerUserServerImpl) SetPermissions(ctx context.Context, request *talkpb.SetPermissionsRequest) (*talkpb.Empty, error) {
	code, err := impl.setPermissions(ctx, request)
	if code != codes.OK {
		return nil, gRPCError(code, err)
	}

	return &talkpb.Empty{}, nil
}

func (impl *servicerUserServerImpl) setPermissions(ctx context.Context, request *talkpb.SetPermissionsRequest) (code codes.Code, err error) {
	code = codes.Unknown

	if request == nil {
		code = codes.InvalidArgument

		return
	}

	err = request.Validate()
	if err != nil {
		return
	}

	_, _, _, admin, _, _, err := impl.tokenHelper.ExtractUserFromGRPCContext(ctx, false)
	if err != nil {
		code = codes.Unauthenticated

		return
	}

	if !admin {
		code = codes.PermissionDenied

		return
	}

	userID, err := simencrypt.DecryptUint64(request.GetUserId())
	if err != nil {
		return
	}

	err = impl.userManager.UpdateUserAllExData(ctx, userID, impl.tokenHelper.GenExData(request.GetActIds(),
		request.GetBizIds()))
	if err != nil {
		return
	}

	code = codes.OK

	return
}

func (impl *servicerUserServerImpl) UserIDS2N(id string) (uint64, error) {
	return simencrypt.DecryptUint64(id)
}
