package transport

import (
	"context"
	"encoding/base64"
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

type State struct {
	Nonce string `json:"nonce"`
	Dest  string `json:"dest"`
}

// Generate generates state and returns base64-encoded form
func (sk *StateKeeper) Generate(dest string) (string, error) {
	sk.Clean()
	u, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	s := &State{
		Nonce: u.String(),
		Dest:  dest,
	}
	sk.Lock()
	sk.states[s.Nonce] = time.Now()
	sk.Unlock()

	j, err := json.Marshal(s)
	if err != nil {
		return "", err
	}

	b := base64.URLEncoding.EncodeToString(j)

	return b, nil
}

// Consume consumes base64-encoded state and returns destination URL
func (sk *StateKeeper) Consume(b string) (string, bool) {
	sk.Clean()
	sk.Lock()
	defer sk.Unlock()

	j, err := base64.URLEncoding.DecodeString(b)
	if err != nil {
		return "", false
	}

	s := &State{}

	err = json.Unmarshal(j, s)
	if err != nil {
		return "", false
	}

	_, ok := sk.states[s.Nonce]
	if ok {
		delete(sk.states, s.Nonce)
	}
	return s.Dest, ok
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

	dest := r.FormValue("dest")

	state, err := stateKeeper.Generate(dest)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to generate state", http.StatusInternalServerError))
		return
	}

	http.Redirect(w, r, a.Conf.OauthConf.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

func (a *App) HandleDiscordCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// verify state

	dest, ok := stateKeeper.Consume(r.FormValue("state"))
	if !ok {
		writeError(ctx, w, perr("state does not match", http.StatusBadRequest))
		return
	}

	// obtain token
	token, err := a.Conf.OauthConf.Exchange(context.Background(), r.FormValue("code"))

	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to obtain discord auth token", http.StatusInternalServerError))
		return
	}

	// obtain user data
	resp, err := a.Conf.OauthConf.Client(context.Background(), token).Get("https://discordapp.com/api/users/@me")

	if err != nil || resp.StatusCode != 200 {
		writeError(ctx, w, perr("failed to obtain discord user data", http.StatusInternalServerError))
		return
	}
	defer resp.Body.Close()

	var discordUserResp discordUserResponse
	err = json.NewDecoder(resp.Body).Decode(&discordUserResp)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to parse discord response", http.StatusInternalServerError))
		return
	}

	uid, err := strconv.ParseInt(discordUserResp.ID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to parse discord response", http.StatusInternalServerError))
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
		writeError(ctx, w, dberr(err))
		return
	}

	if err := a.CC.SetSecureCookie(w, utils.Cookies.Login, service.MapAuthToken(authToken), (int)(a.Conf.SessionExpirationSeconds)); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to set cookie", http.StatusInternalServerError))
		return
	}

	if len(dest) == 0 || !isReturnURLValid(dest) {
		http.Redirect(w, r, "/web/profile", http.StatusFound)
		return
	}

	http.Redirect(w, r, dest, http.StatusFound)
}

func (a *App) HandleLogout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	const msg = "unable to log out, please clear your cookies"
	cookieMap, err := a.CC.GetSecureCookie(r, utils.Cookies.Login)
	if err != nil && !errors.Is(err, http.ErrNoCookie) {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr(msg, http.StatusInternalServerError))
		return
	}

	token, err := service.ParseAuthToken(cookieMap) // TODO move this into the Logout method
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr(msg, http.StatusInternalServerError))
		return
	}

	if err := a.Service.Logout(ctx, token.Secret); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr(msg, http.StatusInternalServerError))
		return
	}

	utils.UnsetCookie(w, utils.Cookies.Login)
	http.Redirect(w, r, "/web", http.StatusFound)
}
