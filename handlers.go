package main

import (
	"context"
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
)

func (a *App) handleRequests(srv *http.Server, router *mux.Router) {
	// oauth
	router.Handle("/auth", http.HandlerFunc(a.HandleDiscordAuth)).Methods("GET")
	router.Handle("/auth/callback", http.HandlerFunc(a.HandleDiscordCallback)).Methods("GET")

	srv.ListenAndServe()
}

func (a *App) HandleDiscordAuth(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, a.conf.OauthConf.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

type DiscordMeResponse struct {
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

	if r.FormValue("state") != state {
		http.Error(w, "state does not match", http.StatusBadRequest)
		return
	}

	token, err := a.conf.OauthConf.Exchange(context.Background(), r.FormValue("code"))

	if err != nil {
		http.Error(w, "failed to obtain discord auth token", http.StatusInternalServerError)
		return
	}

	resp, err := a.conf.OauthConf.Client(context.Background(), token).Get("https://discordapp.com/api/users/@me")

	if err != nil || resp.StatusCode != 200 {
		http.Error(w, "failed to obtain discord user data", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var discordUser DiscordMeResponse
	err = json.NewDecoder(resp.Body).Decode(&discordUser)
	if err != nil {
		http.Error(w, "failed to parse discord response", http.StatusInternalServerError)
		return
	}

	LogCtx(ctx).Infof("%+v\n", discordUser)
}
