package transport

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"net/http"
	"strconv"
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

func (a *App) HandleDiscordAuth(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, a.Conf.OauthConf.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

// TODO provide real, secure oauth state
var state = "random"

func (a *App) HandleDiscordCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// verify state
	if r.FormValue("state") != state {
		http.Error(w, "state does not match", http.StatusBadRequest)
		return
	}

	// obtain token
	token, err := a.Conf.OauthConf.Exchange(context.Background(), r.FormValue("code"))

	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to obtain discord auth token", http.StatusInternalServerError)
		return
	}

	// obtain user data
	resp, err := a.Conf.OauthConf.Client(context.Background(), token).Get("https://discordapp.com/api/users/@me")

	if err != nil || resp.StatusCode != 200 {
		http.Error(w, "failed to obtain discord user data", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var discordUserResp discordUserResponse
	err = json.NewDecoder(resp.Body).Decode(&discordUserResp)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to parse discord response", http.StatusInternalServerError)
		return
	}

	uid, err := strconv.ParseInt(discordUserResp.ID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to parse discord response", http.StatusInternalServerError)
		return
	}

	discordUser := types.DiscordUser{
		ID:            uid,
		Username:      discordUserResp.Username,
		Avatar:        discordUserResp.Avatar,
		Discriminator: discordUserResp.Discriminator,
		PublicFlags:   discordUserResp.PublicFlags,
		Flags:         discordUserResp.Flags,
		Locale:        discordUserResp.Locale,
		MFAEnabled:    discordUserResp.MFAEnabled,
	}

	// save discord user data
	if err := a.DB.StoreDiscordUser(ctx, &discordUser); err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to store discord user", http.StatusInternalServerError)
		return
	}

	// get and save discord user authorization
	isAuthorized, err := a.Bot.IsUserAuthorized(discordUser.ID)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to obtain discord user's roles", http.StatusInternalServerError)
		return
	}
	if err := a.DB.StoreDiscordUserAuthorization(ctx, discordUser.ID, isAuthorized); err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to store discord user's authorization", http.StatusInternalServerError)
		return
	}

	// create cookie and save session
	authToken, err := CreateAuthToken(discordUser.ID)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to generate auth token", http.StatusInternalServerError)
		return
	}
	if err := a.CC.SetSecureCookie(w, Cookies.Login, MapAuthToken(authToken)); err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to set cookie", http.StatusInternalServerError)
		return
	}

	if err = a.DB.StoreSession(ctx, authToken.Secret, discordUser.ID, a.Conf.SessionExpirationSeconds); err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to store session", http.StatusInternalServerError)
	}

	http.Redirect(w, r, "/profile", http.StatusFound)
}

func (a *App) HandleLogout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	const msg = "unable to log out, please clear your cookies and try again"
	cookieMap, err := a.CC.GetSecureCookie(r, Cookies.Login)
	if err != nil && !errors.Is(err, http.ErrNoCookie) {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	token, err := ParseAuthToken(cookieMap)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	if err := a.DB.DeleteSession(ctx, token.Secret); err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	UnsetCookie(w, Cookies.Login)
	http.Redirect(w, r, "/", http.StatusFound)
}