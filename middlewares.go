package main

import (
	"context"
	"net/http"
)

// UserAuth accepts access keys intended for data collectors
func (a *App) UserAuth(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
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

		userID, ok, err := a.GetUIDFromSession(token.Secret)
		if err != nil {
			LogCtx(r.Context()).Error(err)
			http.Error(w, "failed to load session", http.StatusInternalServerError)
			return
		}
		if !ok {
			LogCtx(r.Context()).Error(err)
			http.Error(w, "session expired, please log in to continue", http.StatusUnauthorized)
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), CtxKeys.UserID, userID))
		next(w, r)
	}
}
