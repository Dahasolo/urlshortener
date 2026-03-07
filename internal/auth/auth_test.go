package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecretKey = "test-secret-key"

func TestGenerateUserID(t *testing.T) {
	id, err := GenerateUserID()
	require.NoError(t, err)
	assert.NotEmpty(t, id)
	assert.Len(t, id, 32)
}

func TestGenerateUserID_Unique(t *testing.T) {
	id1, err := GenerateUserID()
	require.NoError(t, err)

	id2, err := GenerateUserID()
	require.NoError(t, err)

	assert.NotEqual(t, id1, id2)
}

func TestSignAndVerifyToken(t *testing.T) {
	userID := "test-user"

	token, err := SignToken(userID, testSecretKey)
	require.NoError(t, err)

	verified, err := VerifyToken(token, testSecretKey)
	require.NoError(t, err)
	assert.Equal(t, userID, verified)
}

func TestVerifyToken_InvalidSignature(t *testing.T) {
	userID := "test-user"

	token, err := SignToken(userID, testSecretKey)
	require.NoError(t, err)

	_, err = VerifyToken(token, "wrong-key")
	assert.Error(t, err)
}

func TestVerifyToken_InvalidFormat(t *testing.T) {
	_, err := VerifyToken("invalid-token", testSecretKey)
	assert.Error(t, err)
}

func TestSetAndGetAuthCookie(t *testing.T) {
	userID := "test-user"

	w := httptest.NewRecorder()
	err := SetAuthCookie(w, userID, testSecretKey)
	require.NoError(t, err)

    resp := w.Result()
    defer resp.Body.Close()
    cookies := resp.Cookies()
    require.Len(t, cookies, 1)

	cookie := cookies[0]
	assert.Equal(t, CookieName, cookie.Name)
	assert.True(t, cookie.HttpOnly)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(cookie)

	extracted, err := GetUserIDFromRequest(req, testSecretKey)
	require.NoError(t, err)
	assert.Equal(t, userID, extracted)
}

func TestGetUserIDFromRequest_NoCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	_, err := GetUserIDFromRequest(req, testSecretKey)
	assert.Error(t, err)
}
