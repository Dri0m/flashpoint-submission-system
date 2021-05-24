package transport

import (
	"bytes"
	"context"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/Masterminds/sprig"
	"html/template"
	"net/http"
)

// RenderTemplates is a helper for rendering templates
func (a *App) RenderTemplates(ctx context.Context, w http.ResponseWriter, r *http.Request, data interface{}, filenames ...string) {
	templates := []string{"templates/base.gohtml", "templates/navbar.gohtml"}
	templates = append(templates, filenames...)
	tmpl, err := template.New("base").Funcs(sprig.FuncMap()).Funcs(template.FuncMap{"boolString": BoolString}).ParseFiles(templates...)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to parse html templates", http.StatusInternalServerError)
		return
	}
	templateBuffer := &bytes.Buffer{}
	err = tmpl.ExecuteTemplate(templateBuffer, "layout", data)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to execute html templates", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(templateBuffer.Bytes()); err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to write page data", http.StatusInternalServerError)
		return
	}
}

// BoolString is a little hack to make handling tri-state bool in go templates trivial
func BoolString(b *bool) string {
	if b == nil {
		return "nil"
	}
	if *b {
		return "true"
	}
	return "false"
}
