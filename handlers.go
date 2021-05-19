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

	//pages
	router.Handle("/", http.HandlerFunc(a.HandleRootPage)).Methods("GET")
	router.Handle("/profile", http.HandlerFunc(a.UserAuth(a.HandleProfilePage))).Methods("GET")
	err := srv.ListenAndServe()
	if err != nil {
		l.Fatal(err)
	}
}

type formattedRole struct {
	Name  string
	Color string
}

type basePageData struct {
	Username                string
	AvatarURL               string
	Roles                   []formattedRole
	IsAuthorizedToUseSystem bool
}

// GetBasePageData loads base user data, does not return error if user is not logged in
func (a *App) GetBasePageData(r *http.Request) (*basePageData, error) {
	ctx := r.Context()
	userID, err := a.GetUserIDFromCookie(r)
	if err != nil {
		LogCtx(ctx).Error(err)
	}

	if userID == "" {
		return nil, nil
	}

	discordUser, err := a.GetDiscordUser(userID)
	if err != nil {
		LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to get user data from db")
	}

	userRoles, err := a.GetFlashpointRolesForUser(userID)
	if err != nil {
		LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load user roles")
	}

	formattedRoles := make([]formattedRole, 0, len(userRoles))
	for _, role := range userRoles {
		formattedRoles = append(formattedRoles, formattedRole{Name: role.Name, Color: fmt.Sprintf("#%06x", role.Color)})
	}

	authorizedRoles := []string{"Administrator", "Moderator", "Curator", "Tester", "Mechanic", "Hunter", "Hacker"}

	isAuthorized := false
	for _, role := range formattedRoles {
		for _, authorizedRole := range authorizedRoles {
			if role.Name == authorizedRole {
				isAuthorized = true
				break
			}
		}
	}

	bpd := &basePageData{
		Username:                discordUser.Username,
		AvatarURL:               fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s", discordUser.ID, discordUser.Avatar),
		Roles:                   formattedRoles,
		IsAuthorizedToUseSystem: isAuthorized,
	}

	return bpd, nil
}

func (a *App) HandleProfilePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bpd, err := a.GetBasePageData(r)
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles("templates/base.gohtml", "templates/profile.gohtml")
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to parse html templates", http.StatusInternalServerError)
		return
	}
	pageData := &bytes.Buffer{}
	err = tmpl.ExecuteTemplate(pageData, "layout", bpd)
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

func (a *App) HandleRootPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	bpd, err := a.GetBasePageData(r)
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles("templates/base.gohtml", "templates/root.gohtml")
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to parse html templates", http.StatusInternalServerError)
		return
	}
	pageData := &bytes.Buffer{}
	err = tmpl.ExecuteTemplate(pageData, "layout", bpd)
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
