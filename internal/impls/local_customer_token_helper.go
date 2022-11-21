package impls

import (
	"context"

	"github.com/sbasestarter/bizinters/userinters"
	anonymousauthenticator "github.com/sbasestarter/userlib/authenticator/anonymous"
	anonymousmanager "github.com/sbasestarter/userlib/manager/anonymous"
	"github.com/sgostarter/i/commerr"
	"github.com/spf13/cast"
	"github.com/zservicer/talkbe/internal/defs"
	"google.golang.org/grpc/metadata"
)

const (
	tokenKeyOnMetadata = "token"

	dKeyUserName = "un"
	dKeyActID    = "a"
	dKeyBizID    = "b"
)

func NewLocalCustomerUserTokenHelper(user userinters.UserCenter) defs.CustomerUserTokenHelper {
	return &localCustomerUserTokenHelperImpl{
		user:    user,
		manager: anonymousmanager.NewManager(),
	}
}

type localCustomerUserTokenHelperImpl struct {
	user    userinters.UserCenter
	manager anonymousmanager.Manager
}

func (impl *localCustomerUserTokenHelperImpl) NewAnonymousAuthenticator(userName, actID, bizID string) userinters.Authenticator {
	return anonymousauthenticator.NewAuthenticator(map[string]interface{}{
		dKeyUserName: userName,
		dKeyActID:    actID,
		dKeyBizID:    bizID,
	})
}

func (impl *localCustomerUserTokenHelperImpl) ExtractTokenFromGRPCContext(ctx context.Context) (token string, err error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		err = commerr.ErrUnauthenticated

		return
	}

	tokens := md.Get(tokenKeyOnMetadata)
	if len(tokens) == 0 || tokens[0] == "" {
		err = commerr.ErrUnauthenticated

		return
	}

	token = tokens[0]

	return
}

func (impl *localCustomerUserTokenHelperImpl) parseDS(exData map[string]interface{}) (userName, actID, bizID string) {
	userName = cast.ToString(exData[dKeyUserName])
	actID = cast.ToString(exData[dKeyActID])
	bizID = cast.ToString(exData[dKeyBizID])

	return
}

func (impl *localCustomerUserTokenHelperImpl) ExplainToken(ctx context.Context, token string,
	renewToken bool) (newToken string, userID uint64, userName, actID, bizID string, err error) {
	newToken, userID, tokenDataList, err := impl.user.CheckToken(ctx, token, renewToken)
	if err != nil {
		return
	}

	ds, err := impl.manager.GetUser(ctx, userID, tokenDataList)
	if err != nil {
		return
	}

	userName, actID, bizID = impl.parseDS(ds)

	return
}

func (impl *localCustomerUserTokenHelperImpl) ExtractUserFromGRPCContext(ctx context.Context,
	renewToken bool) (newToken string, userID uint64, userName, actID, bizID string, err error) {
	token, err := impl.ExtractTokenFromGRPCContext(ctx)
	if err != nil {
		return
	}

	newToken, userID, userName, actID, bizID, err = impl.ExplainToken(ctx, token, renewToken)

	return
}
