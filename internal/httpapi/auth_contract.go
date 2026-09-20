package httpapi

import (
	"context"

	"github.com/fonghehe/vue-h5-template-business-service/internal/auth"
	"github.com/fonghehe/vue-h5-template-business-service/internal/service"
)

// AuthAPI is the consumer-owned interface for replacing auth in HTTP tests.
type AuthAPI interface {
	Login(context.Context, string, string, bool) (service.LoginResult, auth.TokenPair, error)
	Refresh(context.Context, string) (service.LoginResult, auth.TokenPair, error)
	Logout(context.Context, string) error
	VerifyAccessToken(context.Context, string) (*auth.Claims, error)
}
