package main

import (
	"errors"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/gorilla/securecookie"
	"net/http"
	"strconv"
)

// CookieCutter is the cookie handler
type CookieCutter struct {
	Previous *securecookie.SecureCookie
	Current  *securecookie.SecureCookie
}

type cookies struct {
	Login string
}

// Cookies is cookie name enum
var Cookies = cookies{
	Login: "login",
}

// AuthToken is AuthToken
type AuthToken struct {
	Secret string
	UserID string
}

func CreateAuthToken(userID int64) (*AuthToken, error) {
	s, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	return &AuthToken{
		Secret: s.String(),
		UserID: fmt.Sprint(userID),
	}, nil
}

// ParseAuthToken parses map into token
func ParseAuthToken(value map[string]string) (*AuthToken, error) {
	secret, ok := value["secret"]
	if !ok {
		return nil, fmt.Errorf("missing secret")
	}
	userID, ok := value["userID"]
	if !ok {
		return nil, fmt.Errorf("missing userid")
	}
	return &AuthToken{
		Secret: secret,
		UserID: userID,
	}, nil
}

func MapAuthToken(token *AuthToken) map[string]string {
	return map[string]string{"secret": token.Secret, "userID": token.UserID}
}

// SetSecureCookie sets cookie
func (cc *CookieCutter) SetSecureCookie(w http.ResponseWriter, name string, value map[string]string) error {
	encoded, err := securecookie.EncodeMulti(name, value, cc.Current)
	if err != nil {
		return err
	}
	cookie := &http.Cookie{
		Name:     name,
		Value:    encoded,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
	return nil
}

// UnsetCookie unsets cookie
func UnsetCookie(w http.ResponseWriter, name string) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Secure:   true,
		MaxAge:   -1,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
}

// GetSecureCookie gets cookie
func (cc *CookieCutter) GetSecureCookie(r *http.Request, name string) (map[string]string, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return nil, err
	}
	value := make(map[string]string)
	if err := securecookie.DecodeMulti(name, cookie.Value, &value, cc.Current, cc.Previous); err != nil {
		return nil, err
	}
	return value, nil
}

func (a *App) GetUserIDFromCookie(r *http.Request) (int64, error) {
	cookieMap, err := a.CC.GetSecureCookie(r, Cookies.Login)
	if errors.Is(err, http.ErrNoCookie) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	token, err := ParseAuthToken(cookieMap)
	if err != nil {
		return 0, err
	}

	uid, err := strconv.ParseInt(token.UserID, 10, 64)
	if err != nil {
		return 0, err
	}

	return uid, nil
}
