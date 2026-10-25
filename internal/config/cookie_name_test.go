package config

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRefreshCookieNameMatchesHTTPValidation(t *testing.T) {
	for _, name := range []string{"vh5_refresh", "__Host-refresh", "refresh-token", "refresh.token", "refresh/name", "refresh:name", "refresh@name", "refresh[name]", "refresh(name)", "refresh\\name", `refresh"name`, "refresh名", "refresh\x00name", "refresh\x7fname", ""} {
		t.Run(name, func(t *testing.T) {
			cookie := &http.Cookie{Name: name, Value: "example"}
			valid := cookie.Valid() == nil
			cfg := validConfig()
			cfg.RefreshCookieName = name
			err := cfg.Validate()
			if valid && err != nil {
				t.Errorf("Validate(cookie name=%q) error = %v, want nil", name, err)
			}
			if !valid && (err == nil || !strings.Contains(err.Error(), "REFRESH_COOKIE_NAME")) {
				t.Errorf("Validate(cookie name=%q) error = %v, want REFRESH_COOKIE_NAME error before startup", name, err)
			}
			if valid {
				recorder := httptest.NewRecorder()
				http.SetCookie(recorder, cookie)
				if recorder.Header().Get("Set-Cookie") == "" {
					t.Errorf("SetCookie(name=%q) missing cookie after successful validation", name)
				}
			}
		})
	}
}
