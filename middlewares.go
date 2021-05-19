package main

import (
	"context"
	"net/http"
)

// UserAuth accepts valid session cookie
func (a *App) UserAuth(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := a.GetUserIDFromCookie(r)
		if err != nil {
			LogCtx(r.Context()).Error(err)
			http.Error(w, "failed to load session, please clear cookies and try again", http.StatusInternalServerError)
			return
		}
		if userID == "" {
			LogCtx(r.Context()).Error(err)
			http.Error(w, "session expired, please log in to continue", http.StatusUnauthorized)
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), CtxKeys.UserID, userID))
		next(w, r)
	}
}
