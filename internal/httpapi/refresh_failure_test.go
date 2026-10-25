package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/auth"
	"github.com/fonghehe/vue-h5-template-business-service/internal/config"
	"github.com/fonghehe/vue-h5-template-business-service/internal/service"
	"github.com/gin-gonic/gin"
)

type refreshFailureAuth struct {
	AuthAPI
	err error
}

func (a refreshFailureAuth) Refresh(context.Context, string) (service.LoginResult, auth.TokenPair, error) {
	return service.LoginResult{}, auth.TokenPair{}, a.err
}

func TestRefreshPreservesCookieOnTransientFailures(t *testing.T) {
	for _, failure := range []struct {
		name   string
		err    error
		status int
		clear  bool
	}{
		{"unavailable", apierr.Unavailable("temporarily unavailable"), http.StatusServiceUnavailable, false},
		{"internal", errors.New("private storage failure"), http.StatusInternalServerError, false},
		{"deadline", context.DeadlineExceeded, http.StatusGatewayTimeout, false},
		{"cancelled", context.Canceled, http.StatusOK, false},
		{"unauthorized", apierr.Unauthorized("invalid refresh session"), http.StatusUnauthorized, true},
		{"disabled", apierr.Forbidden("account is disabled"), http.StatusForbidden, true},
	} {
		t.Run(failure.name, func(t *testing.T) {
			cfg := config.Load()
			server := &Server{config: cfg, auth: refreshFailureAuth{err: failure.err}}
			router := gin.New()
			router.POST("/api/auth/refresh", server.refresh)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
			request.AddCookie(&http.Cookie{Name: cfg.RefreshCookieName, Value: "existing-session"})
			router.ServeHTTP(recorder, request)
			if recorder.Code != failure.status {
				t.Errorf("refresh(%s) status=%d, want %d", failure.name, recorder.Code, failure.status)
			}
			cookies := recorder.Result().Cookies()
			if failure.clear {
				if len(cookies) != 1 || cookies[0].MaxAge != -1 {
					t.Errorf("refresh(%s) cookies=%v, want expired refresh cookie", failure.name, cookies)
				}
			} else if len(cookies) != 0 {
				t.Errorf("refresh(%s) cookies=%v, want existing cookie preserved for retry", failure.name, cookies)
			}
		})
	}
}
