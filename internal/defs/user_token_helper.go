package defs

import (
	"context"

	"github.com/sbasestarter/bizinters/userinters"
)

type UserTokenExtractor interface {
	ExtractTokenFromGRPCContext(ctx context.Context) (token string, err error)
}

type ServicerUserTokenExplain interface {
	ExplainToken(ctx context.Context, token string, renewToken bool) (newToken string, userID uint64,
		userName string, admin bool, actIDs, bizIDs []string, err error)
}

type ServicerExDataGen interface {
	GenExData(actIDs, bizIDs []string) map[string]interface{}
}

type ServicerUserTokenHelper interface {
	UserTokenExtractor
	ServicerUserTokenExplain
	ServicerExDataGen
	ExtractUserFromGRPCContext(ctx context.Context, renewToken bool) (newToken string, userID uint64,
		userName string, admin bool, actIDs, bizIDs []string, err error)
}

type CustomerUserGenAnonymousAuthenticator interface {
	NewAnonymousAuthenticator(userName, actID, bizID string) userinters.Authenticator
}

type CustomerUserTokenExplain interface {
	ExplainToken(ctx context.Context, token string, renewToken bool) (newToken string, userID uint64,
		userName, actID, bizID string, err error)
}

type CustomerUserTokenHelper interface {
	UserTokenExtractor
	CustomerUserTokenExplain
	CustomerUserGenAnonymousAuthenticator
	ExtractUserFromGRPCContext(ctx context.Context, renewToken bool) (newToken string, userID uint64,
		userName, actID, bizID string, err error)
}
