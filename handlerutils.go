package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
)

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

	if userID == 0 {
		return &basePageData{}, nil
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

// RenderTemplates is a helper for rendering templates
func (a *App) RenderTemplates(ctx context.Context, w http.ResponseWriter, r *http.Request, data interface{}, filenames ...string) {
	templates := []string{"templates/base.gohtml", "templates/navbar.gohtml"}
	templates = append(templates, filenames...)
	tmpl, err := template.ParseFiles(templates...)
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to parse html templates", http.StatusInternalServerError)
		return
	}
	templateBuffer := &bytes.Buffer{}
	err = tmpl.ExecuteTemplate(templateBuffer, "layout", data)
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to execute html templates", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(templateBuffer.Bytes()); err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to write page data", http.StatusInternalServerError)
		return
	}
}
