package auth

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecretKey = "test-secret-key"

// testLogger создаёт тестовый логгер для использования в тестах.
func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

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
	logger := testLogger(t)

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

	extracted, isNew, err := ExtractUserID(req, w, testSecretKey, logger)

	require.NoError(t, err)
	assert.Equal(t, userID, extracted)
	assert.False(t, isNew)
}

func TestExtractUserID_NoCookie(t *testing.T) {
	logger := testLogger(t)
	recorder := httptest.NewRecorder()

	// запрос без куки
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	extractedID, isNew, err := ExtractUserID(req, recorder, testSecretKey, logger)

	require.NoError(t, err)
	assert.NotEmpty(t, extractedID)
	assert.Len(t, extractedID, 32)
	assert.True(t, isNew)

	resp := recorder.Result()
	defer resp.Body.Close()
	cookies := resp.Cookies()
	require.Len(t, cookies, 1)

	cookie := cookies[0]
	assert.Equal(t, CookieName, cookie.Name)
	assert.True(t, cookie.HttpOnly)
}
