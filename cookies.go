package main

import (
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/gorilla/schema"
	"github.com/gorilla/securecookie"
	"net/http"
)

type cookies struct {
	Login string
}

// Cookies is cookie name enum
var Cookies = cookies{
	Login: "login",
}

// TODO rolling keys, see the github page
var cookieHashKey = []byte(RandomString(128)) // TODO persist and generate securely
var cookieBlockKey = []byte(RandomString(32)) // TODO persist and generate securely
var cookieOven = securecookie.New(cookieHashKey, cookieBlockKey)
var decoder = schema.NewDecoder()
var sessionStore = make(map[string]string) // TODO persist

// AuthToken is AuthToken
type AuthToken struct {
	Secret string
}

func CreateAuthToken() (*AuthToken, error) {
	s, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	return &AuthToken{
		Secret: s.String(),
	}, nil
}

// ParseAuthToken parses map into token
func ParseAuthToken(value map[string]string) (*AuthToken, error) {
	secret, ok := value["secret"]
	if !ok {
		return nil, fmt.Errorf("missing secret")
	}
	return &AuthToken{
		Secret: secret,
	}, nil
}

func mapAuthToken(token *AuthToken) map[string]string {
	return map[string]string{"secret": token.Secret}
}

// SetSecureCookie sets cookie
func SetSecureCookie(w http.ResponseWriter, name string, value map[string]string) error {
	encoded, err := cookieOven.Encode(name, value)
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

// GetSecureCookie gets cookie
func GetSecureCookie(r *http.Request, name string) (map[string]string, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return nil, err
	}
	value := make(map[string]string)
	if err := cookieOven.Decode(name, cookie.Value, &value); err != nil {
		return nil, err
	}
	return value, nil
}
