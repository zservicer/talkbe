package impls

import (
	"context"

	"github.com/sbasestarter/bizinters/userinters"
	"github.com/sbasestarter/userlib/manager/userpass"
	"github.com/sgostarter/i/commerr"
	"github.com/spf13/cast"
	"github.com/zservicer/talkbe/internal/defs"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc/metadata"
)

const (
	dKeyPermission   = "permission"
	dKeyExDataActIDs = "actIDs"
	dKeyExDataBizIDs = "bizIDs"
)

func NewLocalServicerUserTokenHelper(user userinters.UserCenter, manager userpass.Manager) defs.ServicerUserTokenHelper {
	return &localServicerUserTokenHelperImpl{
		user:    user,
		manager: manager,
	}
}

type localServicerUserTokenHelperImpl struct {
	user    userinters.UserCenter
	manager userpass.Manager
}

func (impl *localServicerUserTokenHelperImpl) ExtractTokenFromGRPCContext(ctx context.Context) (token string, err error) {
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

func (impl *localServicerUserTokenHelperImpl) ExplainToken(ctx context.Context, token string,
	renewToken bool) (newToken string, userID uint64, userName string, admin bool, actIDs, bizIDs []string, err error) {
	newToken, userID, _, err = impl.user.CheckToken(ctx, token, renewToken)
	if err != nil {
		return
	}

	user, err := impl.manager.GetUser(ctx, userID)
	if err != nil {
		return
	}

	userName = user.UserName
	actIDs = parseIDs(user.ExData[dKeyExDataActIDs])
	bizIDs = parseIDs(user.ExData[dKeyExDataBizIDs])
	admin = cast.ToInt64(user.ExData[dKeyPermission]) > 0

	return
}

func parseIDs(d interface{}) (ids []string) {
	if d == nil {
		return
	}

	if pd, ok := d.(primitive.A); ok {
		for _, i := range pd {
			s, err := cast.ToStringE(i)
			if err == nil {
				ids = append(ids, s)
			}
		}

		return
	}

	ids = cast.ToStringSlice(d)

	return
}

func (impl *localServicerUserTokenHelperImpl) ExtractUserFromGRPCContext(ctx context.Context,
	renewToken bool) (newToken string, userID uint64, userName string, admin bool, actIDs, bizIDs []string, err error) {
	token, err := impl.ExtractTokenFromGRPCContext(ctx)
	if err != nil {
		return
	}

	newToken, userID, userName, admin, actIDs, bizIDs, err = impl.ExplainToken(ctx, token, renewToken)

	return
}

func (impl *localServicerUserTokenHelperImpl) GenExData(actIDs, bizIDs []string) map[string]interface{} {
	m := make(map[string]interface{})
	m[dKeyExDataActIDs] = actIDs
	m[dKeyExDataBizIDs] = bizIDs

	return m
}
