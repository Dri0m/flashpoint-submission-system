package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
)

type DiscordRole struct {
	ID    int64
	Name  string
	Color string
}

type basePageData struct {
	Username                string
	AvatarURL               string
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

	discordUser, err := a.db.GetDiscordUser(userID)
	if err != nil {
		LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to get user data from db")
	}

	isAuthorized, err := a.db.IsDiscordUserAuthorized(userID)
	if err != nil {
		LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load user authorization")
	}

	bpd := &basePageData{
		Username:                discordUser.Username,
		AvatarURL:               fmt.Sprintf("https://cdn.discordapp.com/avatars/%d/%s", discordUser.ID, discordUser.Avatar),
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
