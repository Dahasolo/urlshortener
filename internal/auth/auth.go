package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	CookieName   = "user_auth"
	CookieMaxAge = 30 * 24 * 60 * 60 // 30 дней
)

// GenerateUserID генерирует уникальный ID пользователя.
func GenerateUserID() (string, error) {
	b := make([]byte, 16)

	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("failed to generate user ID: %w", err)
	}

	return hex.EncodeToString(b), nil
}

// SignToken создаёт HMAC-подпись для user_id.
func SignToken(userID, secretKey string) (string, error) {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(userID))
	signature := h.Sum(nil)

	token := userID + "." + hex.EncodeToString(signature)
	return token, nil
}

// VerifyToken проверяет подпись и возвращает user_id.
func VerifyToken(token, secretKey string) (string, error) {
	userID, signatureHex, ok := strings.Cut(token, ".")
	if !ok {
		return "", errors.New("invalid token format")
	}

	receivedSig, err := hex.DecodeString(signatureHex)
	if err != nil {
		return "", fmt.Errorf("invalid signature encoding")
	}

	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(userID))
	expectedSig := h.Sum(nil)

	if !hmac.Equal(receivedSig, expectedSig) {
		return "", fmt.Errorf("invalid signature")
	}

	return string(userID), nil
}

// SetAuthCookie устанавливает подписанную куку в ответ.
func SetAuthCookie(w http.ResponseWriter, userID, secretKey string) error {
	token, err := SignToken(userID, secretKey)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   CookieMaxAge,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// GetUserIDFromRequest извлекает и проверяет user_id из запроса.
func GetUserIDFromRequest(r *http.Request, secretKey string) (string, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return "", fmt.Errorf("no auth cookie")
	}

	userID, err := VerifyToken(cookie.Value, secretKey)
	if err != nil {
		return "", fmt.Errorf("invalid auth token")
	}

	return userID, nil
}
