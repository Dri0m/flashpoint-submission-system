package utils

import (
	"github.com/gorilla/securecookie"
	"net/http"
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

// SetSecureCookie sets cookie
func (cc *CookieCutter) SetSecureCookie(w http.ResponseWriter, name string, value map[string]string, maxAge int) error {
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
		MaxAge:   maxAge,
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
