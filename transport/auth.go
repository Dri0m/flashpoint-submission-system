package transport

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Dri0m/flashpoint-submission-system/service"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/gofrs/uuid"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type discordUserResponse struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Avatar        string `json:"avatar"`
	Discriminator string `json:"discriminator"`
	PublicFlags   int64  `json:"public_flags"`
	Flags         int64  `json:"flags"`
	Locale        string `json:"locale"`
	MFAEnabled    bool   `json:"mfa_enabled"`
}

type StateKeeper struct {
	sync.Mutex
	states            map[string]time.Time
	expirationSeconds uint64
}

func (sk *StateKeeper) Generate() (string, error) {
	sk.Clean()
	u, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	s := u.String()
	sk.Lock()
	sk.states[s] = time.Now()
	sk.Unlock()
	return s, nil
}

func (sk *StateKeeper) Consume(s string) bool {
	sk.Clean()
	sk.Lock()
	defer sk.Unlock()

	_, ok := sk.states[s]
	if ok {
		delete(sk.states, s)
	}
	return ok
}

func (sk *StateKeeper) Clean() {
	sk.Lock()
	defer sk.Unlock()
	for k, v := range sk.states {
		if v.After(v.Add(time.Duration(sk.expirationSeconds))) {
			delete(sk.states, k)
		}
	}
}

var stateKeeper = StateKeeper{
	states:            make(map[string]time.Time),
	expirationSeconds: 30,
}

func (a *App) HandleDiscordAuth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	state, err := stateKeeper.Generate()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(w, perr("failed to generate state", http.StatusInternalServerError))
		return
	}

	http.Redirect(w, r, a.Conf.OauthConf.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

func (a *App) HandleDiscordCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// verify state
	if !stateKeeper.Consume(r.FormValue("state")) {
		writeError(w, perr("state does not match", http.StatusBadRequest))
		return
	}

	// obtain token
	token, err := a.Conf.OauthConf.Exchange(context.Background(), r.FormValue("code"))

	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(w, perr("failed to obtain discord auth token", http.StatusInternalServerError))
		return
	}

	// obtain user data
	resp, err := a.Conf.OauthConf.Client(context.Background(), token).Get("https://discordapp.com/api/users/@me")

	if err != nil || resp.StatusCode != 200 {
		writeError(w, perr("failed to obtain discord user data", http.StatusInternalServerError))
		return
	}
	defer resp.Body.Close()

	var discordUserResp discordUserResponse
	err = json.NewDecoder(resp.Body).Decode(&discordUserResp)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(w, perr("failed to parse discord response", http.StatusInternalServerError))
		return
	}

	uid, err := strconv.ParseInt(discordUserResp.ID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(w, perr("failed to parse discord response", http.StatusInternalServerError))
		return
	}

	discordUser := &types.DiscordUser{
		ID:            uid,
		Username:      discordUserResp.Username,
		Avatar:        discordUserResp.Avatar,
		Discriminator: discordUserResp.Discriminator,
		PublicFlags:   discordUserResp.PublicFlags,
		Flags:         discordUserResp.Flags,
		Locale:        discordUserResp.Locale,
		MFAEnabled:    discordUserResp.MFAEnabled,
	}

	authToken, err := a.Service.SaveUser(ctx, discordUser)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(w, dberr(err))
		return
	}

	if err := a.CC.SetSecureCookie(w, utils.Cookies.Login, service.MapAuthToken(authToken)); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(w, perr("failed to set cookie", http.StatusInternalServerError))
		return
	}

	http.Redirect(w, r, "/profile", http.StatusFound)
}

func (a *App) HandleLogout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	const msg = "unable to log out, please clear your cookies"
	cookieMap, err := a.CC.GetSecureCookie(r, utils.Cookies.Login)
	if err != nil && !errors.Is(err, http.ErrNoCookie) {
		utils.LogCtx(ctx).Error(err)
		writeError(w, perr(msg, http.StatusInternalServerError))
		return
	}

	token, err := service.ParseAuthToken(cookieMap) // TODO move this into the Logout method
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(w, perr(msg, http.StatusInternalServerError))
		return
	}

	if err := a.Service.Logout(ctx, token.Secret); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(w, perr(msg, http.StatusInternalServerError))
		return
	}

	utils.UnsetCookie(w, utils.Cookies.Login)
	http.Redirect(w, r, "/", http.StatusFound)
}
