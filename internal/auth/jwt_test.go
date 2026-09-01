package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSecret   = "unit-test-secret-that-is-long-enough-0001"
	testIssuer   = "vue-h5-template"
	testAudience = "vue-h5-template-api"
)

func newTestIssuer() *Issuer {
	return New(testSecret, testIssuer, testAudience)
}

func issueTestPair(t *testing.T) TokenPair {
	t.Helper()
	pair, err := newTestIssuer().Issue(42, "user", "测试用户", []string{"user", "admin"},
		15*time.Minute, 24*time.Hour, true)
	require.NoError(t, err)
	require.NotEmpty(t, pair.AccessToken)
	require.NotEmpty(t, pair.RefreshToken)
	return pair
}

func TestIssueProducesUsableAccessToken(t *testing.T) {
	pair := issueTestPair(t)

	claims, err := newTestIssuer().Verify(pair.AccessToken, AccessToken)

	require.NoError(t, err)
	userID, err := claims.UserID()
	require.NoError(t, err)
	assert.Equal(t, uint(42), userID)
	assert.Equal(t, "user", claims.Username)
	assert.Equal(t, "测试用户", claims.RealName)
	assert.Equal(t, AccessToken, claims.Type)
	assert.True(t, claims.HasRole("admin"))
	assert.False(t, claims.HasRole("root"))
}

func TestRefreshTokenIsNotAcceptedAsAccessToken(t *testing.T) {
	pair := issueTestPair(t)

	_, err := newTestIssuer().Verify(pair.RefreshToken, AccessToken)

	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestAccessTokenIsNotAcceptedAsRefreshToken(t *testing.T) {
	pair := issueTestPair(t)

	_, err := newTestIssuer().Verify(pair.AccessToken, RefreshToken)

	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestTokenSignedWithAnotherSecretIsRejected(t *testing.T) {
	pair := issueTestPair(t)

	_, err := New("a-completely-different-secret-value-0001", testIssuer, testAudience).
		Verify(pair.AccessToken, AccessToken)

	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestTokenFromAnotherAudienceIsRejected(t *testing.T) {
	pair := issueTestPair(t)

	_, err := New(testSecret, testIssuer, "some-other-audience").
		Verify(pair.AccessToken, AccessToken)

	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestExpiredTokenIsRejected(t *testing.T) {
	issuer := newTestIssuer()
	pair, err := issuer.Issue(1, "user", "u", []string{"user"}, -time.Minute, time.Minute, false)
	require.NoError(t, err)

	_, err = issuer.Verify(pair.AccessToken, AccessToken)

	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestTamperedPayloadIsRejected(t *testing.T) {
	pair := issueTestPair(t)
	parts := strings.Split(pair.AccessToken, ".")
	require.Len(t, parts, 3)
	tampered := parts[0] + "." + parts[1] + ".AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

	_, err := newTestIssuer().Verify(tampered, AccessToken)

	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestAlgNoneTokenIsRejected(t *testing.T) {
	// The classic JWT downgrade attack: a token asking to skip verification.
	token := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{
		Type:             AccessToken,
		RegisteredClaims: jwt.RegisteredClaims{Subject: "1", Issuer: testIssuer},
	})
	unsigned, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = newTestIssuer().Verify(unsigned, AccessToken)

	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestIssueOmitsRefreshTokenWhenNotRequested(t *testing.T) {
	pair, err := newTestIssuer().Issue(1, "user", "u", nil, time.Minute, time.Hour, false)

	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.Empty(t, pair.RefreshToken)
	assert.Equal(t, int64(60), pair.ExpiresIn)
}

func TestBearerParsesAuthorizationHeader(t *testing.T) {
	token, ok := Bearer("Bearer abc.def.ghi")
	require.True(t, ok)
	assert.Equal(t, "abc.def.ghi", token)

	_, ok = Bearer("Basic dXNlcjpwYXNz")
	assert.False(t, ok, "only the Bearer scheme is accepted")

	_, ok = Bearer("Bearer")
	assert.False(t, ok, "a scheme without a token is rejected")

	_, ok = Bearer("")
	assert.False(t, ok)
}

func TestUserIDRejectsNonNumericSubject(t *testing.T) {
	claims := &Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: "not-a-number"}}

	_, err := claims.UserID()

	require.Error(t, err)
}
