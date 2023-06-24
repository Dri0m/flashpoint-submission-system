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
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type discordUserResponse struct {
	ID            string  `json:"id"`
	Username      string  `json:"username"`
	Avatar        string  `json:"avatar"`
	Discriminator string  `json:"discriminator"`
	PublicFlags   int64   `json:"public_flags"`
	Flags         int64   `json:"flags"`
	Locale        string  `json:"locale"`
	MFAEnabled    bool    `json:"mfa_enabled"`
	GlobalName    *string `json:"global_name,omitempty"`
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
	username := discordUserResp.Username
	if discordUserResp.GlobalName != nil && *discordUserResp.GlobalName != "" {
		username = *discordUserResp.GlobalName
	}

	discordUser := &types.DiscordUser{
		ID:            uid,
		Username:      username,
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

func (a *App) HandlePollDeviceAuth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query()
	deviceCode := query.Get("device_code")

	// Get device auth token from storage
	dfToken := a.DFStorage.GetUserAuthTokenByDevice(deviceCode)
	if dfToken == nil {
		writeError(ctx, w, perr("no tokens found", http.StatusBadRequest))
		return
	}

	switch dfToken.FlowState {
	case types.DeviceFlowComplete:
		if dfToken.AuthToken == nil {
			writeError(ctx, w, perr("device auth complete but no token found.", http.StatusInternalServerError))
			return
		}
		// Encode the auth token
		authJson, err := json.Marshal(dfToken.AuthToken)
		if err != nil {
			writeError(ctx, w, perr("failure marshalling token", http.StatusInternalServerError))
			return
		}
		encodedData := base64.StdEncoding.EncodeToString(authJson)
		jsonData := types.DeviceFlowPollResponse{
			Token: encodedData,
		}
		writeResponse(ctx, w, jsonData, http.StatusOK)
		return
	case types.DeviceFlowPending:
		jsonData := types.DeviceFlowPollResponse{
			Error: "authorization_pending",
		}
		writeResponse(ctx, w, jsonData, http.StatusOK)
		return
	case types.DeviceFlowErrorDenied:
		jsonData := types.DeviceFlowPollResponse{
			Error: "access_denied",
		}
		writeResponse(ctx, w, jsonData, http.StatusOK)
		return
	case types.DeviceFlowErrorExpired:
		jsonData := types.DeviceFlowPollResponse{
			Error: "expired_token",
		}
		writeResponse(ctx, w, jsonData, http.StatusOK)
		return
	}

}

func (a *App) HandleNewDeviceToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method == "POST" {
		a.HandlePollDeviceAuth(w, r)
		return
	}

	token, err := a.DFStorage.NewToken()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to create token", http.StatusInternalServerError))
		return
	}

	writeResponse(ctx, w, token, http.StatusOK)
}

func (a *App) HandleApproveDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query()
	code := query.Get("code")

	// Get device auth token from storage
	token, err := a.DFStorage.Get(code)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr(err.Error(), http.StatusBadRequest))
		return
	}

	if r.Method == http.MethodPost {
		// POST User has responded
		action := query.Get("action")
		if action == "approve" {
			// Create a new auth token
			uid := utils.UserID(ctx)
			authToken, err := a.Service.GenAuthToken(ctx, uid)
			if err != nil {
				utils.LogCtx(ctx).Error(err)
				writeError(ctx, w, perr("failed to create new auth token", http.StatusInternalServerError))
				return
			}

			// Save inside device auth
			token.FlowState = types.DeviceFlowComplete
			token.AuthToken = authToken
			err = a.DFStorage.Save(token)
			if err != nil {
				utils.LogCtx(ctx).Error(err)
				writeError(ctx, w, perr("failed to save device token", http.StatusInternalServerError))
				return
			}
		} else if action == "deny" {
			token.FlowState = types.DeviceFlowErrorDenied
			err := a.DFStorage.Save(token)
			if err != nil {
				utils.LogCtx(ctx).Error(err)
				writeError(ctx, w, perr("failed to save token", http.StatusInternalServerError))
				return
			}
		} else {
			writeError(ctx, w, perr("invalid action, must be 'approve' or 'deny'", http.StatusBadRequest))
			return
		}
		// POST Action complete continue to show result same as GET
	}

	// GET Ask for user response
	bpd, err := a.Service.GetBasePageData(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failure getting page data", http.StatusInternalServerError))
		return
	}
	var states = types.DeviceAuthStates{
		Pending:  types.DeviceFlowPending,
		Denied:   types.DeviceFlowErrorDenied,
		Expired:  types.DeviceFlowErrorExpired,
		Complete: types.DeviceFlowComplete,
	}
	pageData := types.DeviceAuthPageData{
		BasePageData: *bpd,
		Token:        token,
		States:       states,
	}

	a.RenderTemplates(ctx, w, r, pageData,
		"templates/device_auth.gohtml")
}

const deviceCodeCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const userCodeCharset = "BCDFGHJKLMNPQRSTVWXZ"

type DeviceFlowUserAuthToken struct {
	AuthToken  string
	DeviceCode string
}

type DeviceFlowStorage struct {
	tokens          map[string]*types.DeviceFlowToken
	authTokens      map[int64]*[]DeviceFlowUserAuthToken
	verificationUrl string
}

func NewDeviceFlowStorage(verificationUrl string) *DeviceFlowStorage {
	return &DeviceFlowStorage{
		tokens:          make(map[string]*types.DeviceFlowToken),
		authTokens:      make(map[int64]*[]DeviceFlowUserAuthToken),
		verificationUrl: verificationUrl,
	}
}

func (s *DeviceFlowStorage) GetUserAuthTokenByDevice(deviceCode string) *types.DeviceFlowToken {
	var dfToken *types.DeviceFlowToken
	for _, token := range s.tokens {
		if token.DeviceCode == deviceCode {
			dfToken = token
		}
	}

	return dfToken
}

func (s *DeviceFlowStorage) SaveUserAuthToken(uid int64, token string, deviceCode string) {
	userToken := DeviceFlowUserAuthToken{
		AuthToken:  token,
		DeviceCode: deviceCode,
	}
	*s.authTokens[uid] = append(*s.authTokens[uid], userToken)
}

func (s *DeviceFlowStorage) GetUserAuthTokens(uid int64) *[]DeviceFlowUserAuthToken {
	return s.authTokens[uid]
}

func (s *DeviceFlowStorage) NewToken() (*types.DeviceFlowToken, error) {
	// Generate the code
	deviceCode := make([]byte, 32)
	for i := range deviceCode {
		deviceCode[i] = deviceCodeCharset[rand.Intn(len(deviceCodeCharset))]
	}

	userCode := make([]byte, 32)
	for i := range userCode {
		userCode[i] = userCodeCharset[rand.Intn(len(userCodeCharset))]
	}

	expiresAt := time.Now()
	expiresAt = expiresAt.Add(900 * time.Second)

	token := types.DeviceFlowToken{
		DeviceCode:      string(deviceCode),
		UserCode:        string(userCode),
		VerificationURL: s.verificationUrl,
		ExpiresIn:       900,
		ExpiresAt:       expiresAt,
		Interval:        3,
		FlowState:       types.DeviceFlowPending,
	}

	err := s.Save(&token)
	if err != nil {
		return &token, err
	}

	return &token, nil
}

func (s *DeviceFlowStorage) Save(token *types.DeviceFlowToken) error {
	s.tokens[token.UserCode] = token
	return nil
}

func (s *DeviceFlowStorage) Get(userCode string) (*types.DeviceFlowToken, error) {
	token, found := s.tokens[userCode]
	if !found {
		return nil, errors.New("device code not found")
	}
	if time.Now().After(token.ExpiresAt) {
		return nil, errors.New("device code has expired")
	}
	return token, nil
}

func (s *DeviceFlowStorage) Delete(deviceCode string) {
	delete(s.tokens, deviceCode)
}

func (s *DeviceFlowStorage) Cleanup() {
	for deviceCode, token := range s.tokens {
		if time.Now().After(token.ExpiresAt) {
			s.Delete(deviceCode)
		}
	}
}
