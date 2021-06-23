package transport

import (
	"bytes"
	"context"
	"errors"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/service"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/Masterminds/sprig"
	"github.com/kofalt/go-memoize"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var cache = memoize.NewMemoizer(10*time.Minute, 60*time.Minute)

// RenderTemplates is a helper for rendering templates
func (a *App) RenderTemplates(ctx context.Context, w http.ResponseWriter, r *http.Request, data interface{}, filenames ...string) {
	templates := []string{"templates/base.gohtml", "templates/navbar.gohtml"}
	templates = append(templates, filenames...)

	t := template.New("base").Funcs(sprig.FuncMap()).Funcs(template.FuncMap{
		"boolString":         BoolString,
		"unpointify":         utils.Unpointify,
		"isStaff":            constants.IsStaff,
		"isTrialCurator":     constants.IsTrialCurator,
		"isDeletor":          constants.IsDeletor,
		"isDecider":          constants.IsDecider,
		"isAdder":            constants.IsAdder,
		"isInAudit":          constants.IsInAudit,
		"megabytify":         utils.Megabytify,
		"splitMultilineText": utils.SplitMultilineText,
	})

	parse := func() (interface{}, error) {
		return t.ParseFiles(templates...)
	}

	result, err, cached := cache.Memoize(strings.Join(templates, ","), parse)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to parse html templates", http.StatusInternalServerError)
		return
	}

	if cached {
		utils.LogCtx(ctx).Error("using cached template files")
	} else {
		utils.LogCtx(ctx).Error("reading fresh template files from the disk")
	}

	tmpl := result.(*template.Template)

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

func (a *App) GetUserIDFromCookie(r *http.Request) (int64, error) {
	cookieMap, err := a.CC.GetSecureCookie(r, utils.Cookies.Login)
	if errors.Is(err, http.ErrNoCookie) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	token, err := service.ParseAuthToken(cookieMap)
	if err != nil {
		return 0, err
	}

	uid, err := strconv.ParseInt(token.UserID, 10, 64)
	if err != nil {
		return 0, err
	}

	return uid, nil
}

func (a *App) GetSecretFromCookie(r *http.Request) (string, error) {
	cookieMap, err := a.CC.GetSecureCookie(r, utils.Cookies.Login)
	if errors.Is(err, http.ErrNoCookie) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	token, err := service.ParseAuthToken(cookieMap)
	if err != nil {
		return "", err
	}

	return token.Secret, nil
}

func writeError(w http.ResponseWriter, err error) {
	ufe := &constants.PublicError{}
	if errors.As(err, ufe) {
		http.Error(w, ufe.Msg, ufe.Status)
	} else {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func perr(msg string, status int) error {
	return constants.PublicError{Msg: msg, Status: status}
}
