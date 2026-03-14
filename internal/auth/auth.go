package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

const (
	CookieName   = "user_auth"
	CookieMaxAge = 30 * 24 * 60 * 60 // 30 дней
)

var (
	ErrNoCookie        = errors.New("no auth cookie")
	ErrInvalidToken    = errors.New("invalid token format")
	ErrGenerateFailed  = errors.New("failed to generate user ID")
	ErrSetCookieFailed = errors.New("failed to set auth cookie")
)

// GenerateUserID генерирует уникальный ID пользователя.
func GenerateUserID() (string, error) {
	b := make([]byte, 16)

	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("%v: %w", ErrGenerateFailed, err)
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
		return "", fmt.Errorf("%w: invalid format", ErrInvalidToken)
	}

	receivedSig, err := hex.DecodeString(signatureHex)
	if err != nil {
		return "", fmt.Errorf("%w: invalid signature encoding: %v", ErrInvalidToken, err)
	}

	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(userID))
	expectedSig := h.Sum(nil)

	if !hmac.Equal(receivedSig, expectedSig) {
		return "", fmt.Errorf("%w: invalid signature", ErrInvalidToken)
	}

	return string(userID), nil
}

// SetAuthCookie устанавливает подписанную куку в ответ.
func SetAuthCookie(w http.ResponseWriter, userID, secretKey string) error {
	token, err := SignToken(userID, secretKey)
	if err != nil {
		return fmt.Errorf("%v: %w", ErrSetCookieFailed, err)
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

// ExtractUserID извлекает user_id из куки.
func ExtractUserID(r *http.Request, w http.ResponseWriter, secretKey string, logger *slog.Logger) (string, bool, error) {
	cookie, err := r.Cookie(CookieName)

	// Если куки нет - создаём нового пользователя
	if errors.Is(err, http.ErrNoCookie) {
		userID, genErr := GenerateUserID()
		if genErr != nil {
			logger.Error("failed to generate user ID", "error", genErr)
			return "", false, genErr
		}
		if setErr := SetAuthCookie(w, userID, secretKey); setErr != nil {
			logger.Error("failed to set auth cookie", "error", setErr)
			return "", false, setErr
		}
		return userID, true, nil
	}

	// Ошибка чтения куки
	if err != nil {
		logger.Warn("failed to read cookie", "error", err)
		return "", false, fmt.Errorf("failed to read cookie: %w", err)
	}

	// Если кука есть - проверяем подпись
	userID, verifyErr := VerifyToken(cookie.Value, secretKey)
	if verifyErr != nil {
		return "", false, ErrInvalidToken
	}

	return userID, false, nil
}
