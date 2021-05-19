package main

import (
	"bytes"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"html/template"
	"net/http"
)

func (a *App) handleRequests(l *logrus.Logger, srv *http.Server, router *mux.Router) {
	// oauth
	router.Handle("/auth", http.HandlerFunc(a.HandleDiscordAuth)).Methods("GET")
	router.Handle("/auth/callback", http.HandlerFunc(a.HandleDiscordCallback)).Methods("GET")

	// logout
	router.Handle("/logout", http.HandlerFunc(a.HandleLogout)).Methods("GET")

	//file server
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	//home
	router.Handle("/", http.HandlerFunc(a.HandleRootPage)).Methods("GET")
	err := srv.ListenAndServe()
	if err != nil {
		l.Fatal(err)
	}
}

type basePageData struct {
	Username  string
	AvatarURL string
}

func (a *App) GetBasePageData(userID string) (*basePageData, error) {
	discordUser, err := a.GetDiscordUser(userID)
	if err != nil {
		return nil, err
	}
	return &basePageData{
		Username:  discordUser.Username,
		AvatarURL: fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s", discordUser.ID, discordUser.Avatar),
	}, nil
}

type homePageData struct {
	basePageData
}

func (a *App) HandleRootPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := a.GetUserIDFromCookie(r)
	if err != nil {
		LogCtx(ctx).Error(err)
	}

	hpd := homePageData{}
	if userID != "" {
		bpd, err := a.GetBasePageData(userID)
		if err != nil {
			LogCtx(ctx).Error(err)
			http.Error(w, "failed to load user data", http.StatusInternalServerError)
			return
		}

		hpd.basePageData = *bpd
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
