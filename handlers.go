package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"html/template"
	"net/http"
)

func (a *App) handleRequests(srv *http.Server, router *mux.Router) {
	// oauth
	router.Handle("/auth", http.HandlerFunc(a.HandleDiscordAuth)).Methods("GET")
	router.Handle("/auth/callback", http.HandlerFunc(a.HandleDiscordCallback)).Methods("GET")

	//file server
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	//home
	router.Handle("/", http.HandlerFunc(a.HandleRootPage)).Methods("GET")
	srv.ListenAndServe()
}

type homePageData struct {
	Username  string
	AvatarURL string
}

func (a *App) HandleRootPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := a.GetUserIDFromCookie(r)
	if err != nil {
		LogCtx(ctx).Error(err)
	}

	hpd := homePageData{}

	if userID != "" {
		discordUser, err := a.GetDiscordUser(userID)
		if err != nil {
			LogCtx(ctx).Error(err)
			http.Error(w, "failed to load user data", http.StatusInternalServerError)
			return
		}
		hpd.Username = discordUser.Username
		hpd.AvatarURL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s", discordUser.ID, discordUser.Avatar)
	}

	//userRoles, err := a.GetFlashpointRolesForUser(userID)
	//if err != nil {
	//	LogCtx(ctx).Error(err)
	//	http.Error(w, "failed to load user roles", http.StatusInternalServerError)
	//	return
	//}
	//
	//roles := ""
	//for _, role := range userRoles {
	//	LogCtx(ctx).Infof("%+v", role)
	//	roles += fmt.Sprintf("<b><p style='color:#%06x;'>%s </p></b>", role.Color, role.Name)
	//}
	//
	//w.Header().Set("Content-Type", "text/html; charset=utf-8")
	//w.Write([]byte(fmt.Sprintf("<html style='background-color:#333; color:#eee'>hello %s, this is your home now. <img src='https://cdn.discordapp.com/avatars/%s/%s'><br>roles: %s<br></html>",
	//	discordUser.Username, discordUser.ID, discordUser.Avatar, roles)))

	tmpl, err := template.ParseFiles("templates/base.gohtml")
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to parse html templates", http.StatusInternalServerError)
		return
	}
	pageData := &bytes.Buffer{}
	err = tmpl.ExecuteTemplate(pageData, "layout", hpd)
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to execute html templates", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(pageData.Bytes()); err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to write page data", http.StatusInternalServerError)
		return
	}
}

func (a *App) HandleDiscordAuth(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, a.conf.OauthConf.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

type DiscordUserResponse struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Avatar        string `json:"avatar"`
	Discriminator string `json:"discriminator"`
	PublicFlags   int    `json:"public_flags"`
	Flags         int    `json:"flags"`
	Locale        string `json:"locale"`
	MFAEnabled    bool   `json:"mfa_enabled"`
}

// TODO provide real, secure oauth state
var state = "random"

func (a *App) HandleDiscordCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// verify state
	if r.FormValue("state") != state {
		http.Error(w, "state does not match", http.StatusBadRequest)
		return
	}

	// obtain token
	token, err := a.conf.OauthConf.Exchange(context.Background(), r.FormValue("code"))

	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to obtain discord auth token", http.StatusInternalServerError)
		return
	}

	// obtain user data
	resp, err := a.conf.OauthConf.Client(context.Background(), token).Get("https://discordapp.com/api/users/@me")

	if err != nil || resp.StatusCode != 200 {
		http.Error(w, "failed to obtain discord user data", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var discordUser DiscordUserResponse
	err = json.NewDecoder(resp.Body).Decode(&discordUser)
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to parse discord response", http.StatusInternalServerError)
		return
	}
	LogCtx(ctx).Infof("%+v\n", discordUser)

	// save discord user data
	if err := a.StoreDiscordUser(&discordUser); err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to store discord user", http.StatusInternalServerError)
		return
	}

	// create cookie and save session
	authToken, err := CreateAuthToken()
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to generate auth token", http.StatusInternalServerError)
		return
	}
	if err := SetSecureCookie(w, Cookies.Login, mapAuthToken(authToken)); err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to set cookie", http.StatusInternalServerError)
		return
	}

	if err = a.StoreSession(authToken.Secret, discordUser.ID); err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to store session", http.StatusInternalServerError)
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}
