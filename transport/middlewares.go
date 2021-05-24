package transport

import (
	"context"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"net/http"
)

// UserAuthentication accepts valid session cookie
func (a *App) UserAuthentication(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := a.GetUserIDFromCookie(r)
		if err != nil {
			utils.LogCtx(r.Context()).Error(err)
			http.Error(w, "failed to load session, please clear cookies and try again", http.StatusInternalServerError)
			return
		}
		if userID == 0 {
			utils.LogCtx(r.Context()).Error(err)
			http.Error(w, "please log in to continue", http.StatusUnauthorized)
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), utils.CtxKeys.UserID, userID))
		next(w, r)
	}
}

// UserAuthorization accepts user that is authorized
func (a *App) UserAuthorization(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := a.GetUserIDFromCookie(r)
		if err != nil {
			utils.LogCtx(r.Context()).Error(err)
			http.Error(w, "failed to load session, please clear cookies and try again", http.StatusInternalServerError)
			return
		}
		if userID == 0 {
			utils.LogCtx(r.Context()).Error(err)
			http.Error(w, "please log in to continue", http.StatusUnauthorized)
			return
		}
		isAuthorized, err := a.DB.IsDiscordUserAuthorized(r.Context(), userID)
		if err != nil {
			utils.LogCtx(r.Context()).Error(err)
			http.Error(w, "failed to load user authorization", http.StatusInternalServerError)
			return
		}

		if !isAuthorized {
			utils.LogCtx(r.Context()).Info("attempt to access page without proper authorization")
			http.Error(w, "you do not have the proper authorization to access this page", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}
