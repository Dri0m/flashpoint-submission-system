package main

import (
	"context"
	"net/http"
)

// UserAuth accepts access keys intended for data collectors
func UserAuth(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		cookieMap, err := GetSecureCookie(r, Cookies.Login)
		if err != nil {
			LogCtx(r.Context()).Error(err)
			http.Error(w, "please log in to continue", http.StatusUnauthorized)
			return
		}

		token, err := ParseAuthToken(cookieMap)
		if err != nil {
			LogCtx(r.Context()).Error(err)
			http.Error(w, "please log in to continue", http.StatusUnauthorized)
			return
		}

		userID, ok := sessionStore[token.Secret]
		if !ok {
			LogCtx(r.Context()).Error(err)
			http.Error(w, "please log in to continue", http.StatusUnauthorized)
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), CtxKeys.UserID, userID))
		next(w, r)
	}
}
