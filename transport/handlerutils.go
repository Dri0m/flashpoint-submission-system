package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/service"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/Masterminds/sprig"
	"github.com/kofalt/go-memoize"
)

var templateCache = memoize.NewMemoizer(10*time.Minute, 60*time.Minute)

// RenderTemplates is a helper for rendering templates
func (a *App) RenderTemplates(ctx context.Context, w http.ResponseWriter, r *http.Request, data interface{}, filenames ...string) {
	templates := []string{"templates/base.gohtml", "templates/navbar.gohtml"}
	templates = append(templates, filenames...)

	t := template.New("base").Funcs(sprig.FuncMap()).Funcs(template.FuncMap{
		"boolString":                    BoolString,
		"unpointify":                    utils.Unpointify,
		"isStaff":                       constants.IsStaff,
		"isTrialCurator":                constants.IsTrialCurator,
		"isDeleter":                     constants.IsDeleter,
		"isDecider":                     constants.IsDecider,
		"isAdder":                       constants.IsAdder,
		"isInAudit":                     constants.IsInAudit,
		"isGod":                         constants.IsGod,
		"sizeToString":                  utils.SizeToString,
		"splitMultilineText":            utils.SplitMultilineText,
		"capitalizeAscii":               utils.CapitalizeASCII,
		"parseMetaTags":                 parseMetaTags,
		"submissionsShowPreviousButton": submissionsShowPreviousButton,
		"submissionsShowNextButton":     submissionsShowNextButton,
		"capString":                     capString,
		"er":                            equalReference,
		"ner":                           notEqualReference,
		"localeNum":                     localeNum,
	})

	parse := func() (interface{}, error) {
		return t.ParseFiles(templates...)
	}

	var result interface{}
	var err error
	cached := false

	if a.Conf.IsDev {
		result, err = parse()
	} else {
		result, err, cached = templateCache.Memoize(strings.Join(templates, ","), parse)
	}
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to parse html templates", http.StatusInternalServerError)
		return
	}

	utils.LogCtx(ctx).WithField("cached", utils.BoolToString(cached)).Debug("executing template files")

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

func (a *App) GetSecretFromCookie(ctx context.Context, r *http.Request) (string, error) {
	cookieMap, err := a.CC.GetSecureCookie(r, utils.Cookies.Login)
	if err != nil {
		if err == http.ErrNoCookie {
			return "", err
		}
		utils.LogCtx(ctx).Error(err)
		return "", err
	}

	token, err := service.ParseAuthToken(cookieMap)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return "", err
	}

	return token.Secret, nil
}

func writeResponse(ctx context.Context, w http.ResponseWriter, data interface{}, status int) {
	requestType := utils.RequestType(ctx)
	if requestType == "" {
		utils.LogCtx(ctx).Panic("request type not set")
		return
	}

	switch requestType {
	case constants.RequestJSON, constants.RequestData, constants.RequestWeb:
		w.WriteHeader(status)
		if data != nil {
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(data)
			if err != nil {
				utils.LogCtx(ctx).Error(err)
				if errors.Is(err, syscall.ECONNRESET) {
					return
				}
				if errors.Is(err, syscall.EPIPE) {
					return
				}
				writeError(ctx, w, err)
			}
		}
	default:
		utils.LogCtx(ctx).Panic("unsupported request type")
	}
}

func writeError(ctx context.Context, w http.ResponseWriter, err error) {
	ufe := &constants.PublicError{}
	if errors.As(err, ufe) {
		writeResponse(ctx, w, presp(ufe.Msg, ufe.Status), ufe.Status)
	} else {
		msg := http.StatusText(http.StatusInternalServerError)
		writeResponse(ctx, w, presp(msg, http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func perr(msg string, status int) error {
	return constants.PublicError{Msg: msg, Status: status}
}

func dberr(err error) error {
	return constants.DatabaseError{Err: err}
}

func presp(msg string, status int) constants.PublicResponse {
	return constants.PublicResponse{Msg: &msg, Status: status}
}

func parseMetaTags(rawTags string, tagList []types.Tag) []types.Tag {
	splitTags := strings.Split(rawTags, ";")
	normalizedTags := make([]string, 0, len(splitTags))
	for _, tag := range splitTags {
		normalizedTags = append(normalizedTags, strings.ToLower(strings.TrimSpace(tag)))
	}

	// TODO this map is being remade every time for no reason really
	tagMap := make(map[string]string)
	for _, tag := range tagList {
		tagMap[strings.ToLower(strings.TrimSpace(tag.Name))] = tag.Description
	}

	result := make([]types.Tag, 0, len(normalizedTags))
	for i, tag := range normalizedTags {
		resultTag := types.Tag{
			Name:        splitTags[i],
			Description: "Unknown tag.",
		}
		if desc, ok := tagMap[tag]; ok {
			resultTag.Description = desc
		}
		result = append(result, resultTag)
	}

	return result
}

func submissionsShowPreviousButton(page *int64) bool {
	return !(page == nil || *page < 2)
}

// TODO doesn't work correctly when the total number of results is divisible by perPage
func submissionsShowNextButton(submissionCount int, perPage *int64) bool {
	var currentPerPage int64 = 100
	if perPage != nil {
		currentPerPage = *perPage
	}
	return (int64)(submissionCount) == currentPerPage
}

func capString(maxLen int, s *string) string {
	if s == nil {
		return "<nil>"
	}
	str := *s
	if len(str) <= 3 {
		return *s
	}
	if len(str) <= maxLen {
		return str
	}
	return str[:maxLen-3] + "..."
}

func equalReference(ref *string, str string) bool {
	if ref != nil {
		return *ref == str
	} else {
		return str == ""
	}
}

func notEqualReference(ref *string, str string) bool {
	if ref != nil {
		return *ref != str
	} else {
		return str != ""
	}
}

func localeNum(ref *int64) string {
	p := message.NewPrinter(language.English)
	s := p.Sprintf("%d\n", *ref)
	return s
}

func isReturnURLValid(s string) bool {
	return len(s) > 0 && strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//")
}
