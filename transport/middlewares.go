package transport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/service"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/gorilla/mux"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (a *App) RequestWeb(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		next(w, r.WithContext(context.WithValue(r.Context(), utils.CtxKeys.RequestType, constants.RequestWeb)))
	}
}

func (a *App) RequestJSON(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		next(w, r.WithContext(context.WithValue(r.Context(), utils.CtxKeys.RequestType, constants.RequestJSON)))
	}
}

func (a *App) RequestData(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		next(w, r.WithContext(context.WithValue(r.Context(), utils.CtxKeys.RequestType, constants.RequestData)))
	}
}

// UserAuthMux takes many authorization middlewares and accepts if any of them does not return error
func (a *App) UserAuthMux(next func(http.ResponseWriter, *http.Request), authorizers ...func(*http.Request, int64) (bool, error)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		handleAuthErr := func() {
			utils.UnsetCookie(w, utils.Cookies.Login)
			rt := utils.RequestType(ctx)

			switch rt {
			case constants.RequestWeb:
				returnURL := r.URL.Path
				if len(r.URL.RawQuery) > 0 {
					returnURL += "?" + r.URL.RawQuery
				}
				if len(r.URL.RawFragment) > 0 {
					returnURL += "#" + r.URL.RawFragment
				}
				returnURL = url.QueryEscape(returnURL)
				http.Redirect(w, r, fmt.Sprintf("/auth?dest=%s", returnURL), http.StatusFound)
			case constants.RequestData, constants.RequestJSON:
				writeError(ctx, w, perr("failed to parse cookie, please clear your cookies and try again", http.StatusUnauthorized))
			default:
				utils.LogCtx(ctx).Panic("request type not set")
			}

		}

		var secret string
		var err error
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			// try bearer token
			// split the header at the space character
			authHeaderParts := strings.Split(authHeader, " ")
			if len(authHeaderParts) != 2 || authHeaderParts[0] != "Bearer" {
				handleAuthErr()
				return
			}
			decodedBytes, err := base64.StdEncoding.DecodeString(authHeaderParts[1])
			if err != nil {
				handleAuthErr()
				return
			}
			var tokenMap map[string]string
			err = json.Unmarshal(decodedBytes, &tokenMap)
			if err != nil {
				handleAuthErr()
				return
			}
			token, err := service.ParseAuthToken(tokenMap)
			if err != nil {
				handleAuthErr()
				return
			}
			secret = token.Secret
		} else {
			// try cookie
			secret, err = a.GetSecretFromCookie(ctx, r)
			if err != nil {
				handleAuthErr()
				return
			}
		}

		uid, ok, err := a.Service.GetUIDFromSession(ctx, secret)
		if err != nil {
			handleAuthErr()
			return
		}
		if !ok {
			handleAuthErr()
			return
		}

		if len(authorizers) == 0 {
			r = r.WithContext(context.WithValue(ctx, utils.CtxKeys.UserID, uid))
			next(w, r)
			return
		}

		allOk := true

		for _, authorizer := range authorizers {
			ok, err := authorizer(r, uid)
			if err != nil {
				utils.LogCtx(ctx).Error(err)
				writeError(ctx, w, perr("failed to verify authority", http.StatusInternalServerError))
				return
			}
			if !ok {
				allOk = false
				break
			}
		}

		if allOk {
			r = r.WithContext(context.WithValue(ctx, utils.CtxKeys.UserID, uid))
			next(w, r)
			return
		}

		utils.LogCtx(ctx).Debug("unauthorized attempt")
		writeError(ctx, w, perr("you do not have the proper authorization to access this page", http.StatusUnauthorized))
	}
}

// UserHasAllRoles accepts user that has at least all requiredRoles
func (a *App) UserHasAllRoles(r *http.Request, uid int64, requiredRoles []string) (bool, error) {
	ctx := r.Context()

	getUserRoles := func() (interface{}, error) {
		return a.Service.GetUserRoles(ctx, uid)
	}

	userRoles, err, cached := a.authMiddlewareCache.Memoize(fmt.Sprintf("getUserRoles-%d", uid), getUserRoles)
	if err != nil {
		return false, err
	}

	utils.LogCtx(ctx).WithField("cached", utils.BoolToString(cached))

	isAuthorized := true

	for _, role := range userRoles.([]string) {
		foundRole := false
		for _, requiredRole := range requiredRoles {
			if role == requiredRole {
				foundRole = true
				break
			}
		}
		if !foundRole {
			isAuthorized = false
			break
		}
	}

	if !isAuthorized {
		return false, nil
	}

	return true, nil
}

// UserHasAnyRole accepts user that has at least one of requiredRoles
func (a *App) UserHasAnyRole(r *http.Request, uid int64, roles []string) (bool, error) {
	ctx := r.Context()

	getUserRoles := func() (interface{}, error) {
		return a.Service.GetUserRoles(ctx, uid)
	}

	userRoles, err, cached := a.authMiddlewareCache.Memoize(fmt.Sprintf("getUserRoles-%d", uid), getUserRoles)
	if err != nil {
		return false, err
	}

	utils.LogCtx(ctx).WithField("cached", utils.BoolToString(cached)).Debug("getting user roles")

	isAuthorized := constants.HasAnyRole(userRoles.([]string), roles)
	if !isAuthorized {
		return false, nil
	}

	return true, nil
}

// UserOwnsResource accepts user that owns given resource(s)
func (a *App) UserOwnsResource(r *http.Request, uid int64, resourceKey string) (bool, error) {
	ctx := r.Context()

	searchSubmissionBySID := func(sid int64) func() (interface{}, error) {
		return func() (interface{}, error) {
			s, _, err := a.Service.SearchSubmissions(ctx, &types.SubmissionsFilter{SubmissionIDs: []int64{sid}})
			return s, err
		}
	}

	getSubmissionFileByFID := func(fid int64) func() (interface{}, error) {
		return func() (interface{}, error) {
			return a.Service.GetSubmissionFiles(ctx, []int64{fid})
		}
	}

	if resourceKey == constants.ResourceKeySubmissionID {
		params := mux.Vars(r)
		submissionID := params[constants.ResourceKeySubmissionID]
		sid, err := strconv.ParseInt(submissionID, 10, 64)
		if err != nil {
			return false, fmt.Errorf("invalid submission id")
		}

		submissions, err, cached := a.authMiddlewareCache.Memoize(fmt.Sprintf("searchSubmissionBySID-%d", sid), searchSubmissionBySID(sid))
		if err != nil {
			return false, err
		}
		utils.LogCtx(ctx).WithField("cached", utils.BoolToString(cached)).Debug("searching submission by submission id")

		if len(submissions.([]*types.ExtendedSubmission)) == 0 {
			return false, fmt.Errorf("submission with id %d not found", sid)
		}

		s := submissions.([]*types.ExtendedSubmission)[0]
		if s.SubmitterID != uid {
			return false, nil
		}

	} else if resourceKey == constants.ResourceKeySubmissionIDs {
		params := mux.Vars(r)
		submissionIDs := strings.Split(params["submission-ids"], ",")
		sids := make([]int64, 0, len(submissionIDs))

		for _, submissionID := range submissionIDs {
			sid, err := strconv.ParseInt(submissionID, 10, 64)
			if err != nil {
				return false, fmt.Errorf("invalid submission id")
			}
			sids = append(sids, sid)
		}

		for _, sid := range sids {
			submissions, err, cached := a.authMiddlewareCache.Memoize(fmt.Sprintf("searchSubmissionBySID-%d", sid), searchSubmissionBySID(sid))
			if err != nil {
				return false, err
			}
			utils.LogCtx(ctx).WithField("cached", utils.BoolToString(cached)).Debug("searching submission by submission id")

			if len(submissions.([]*types.ExtendedSubmission)) == 0 {
				return false, fmt.Errorf("submission with id %d not found", sid)
			}

			submission := submissions.([]*types.ExtendedSubmission)[0]

			if submission.SubmitterID != uid {
				return false, nil
			}
		}

	} else if resourceKey == constants.ResourceKeyFileID {
		params := mux.Vars(r)
		submissionID := params[constants.ResourceKeyFileID]
		fid, err := strconv.ParseInt(submissionID, 10, 64)
		if err != nil {
			return false, nil
		}

		submissionFiles, err, cached := a.authMiddlewareCache.Memoize(fmt.Sprintf("getSubmissionFileByFID-%d", fid), getSubmissionFileByFID(fid))
		if err != nil {
			return false, err
		}
		utils.LogCtx(ctx).WithField("cached", utils.BoolToString(cached)).Debug("searching submission file by submission file id")

		sf := submissionFiles.([]*types.SubmissionFile)[0]
		if sf.SubmitterID != uid {
			return false, nil
		}

	} else if resourceKey == constants.ResourceKeyFixID {
		params := mux.Vars(r)
		submissionID := params[constants.ResourceKeyFixID]
		fid, err := strconv.ParseInt(submissionID, 10, 64)
		if err != nil {
			return false, nil
		}

		fix, err := a.Service.GetFixByID(ctx, fid)
		if err != nil {
			return false, nil
		}

		if fix.AuthorID != uid {
			return false, nil
		}

	} else {
		return false, fmt.Errorf("invalid resource")
	}

	return true, nil
}

// IsUserWithinResourceLimit accepts if user has no more than given amount of given resource(s)
func (a *App) IsUserWithinResourceLimit(r *http.Request, uid int64, resourceKey string, resourceAmount int) (bool, error) {
	ctx := r.Context()

	if resourceKey == constants.ResourceKeySubmissionID {
		submissions, _, err := a.Service.SearchSubmissions(ctx, &types.SubmissionsFilter{SubmitterID: &uid, DistinctActionsNot: []string{constants.ActionReject}}) // don't count rejected submissions
		if err != nil {
			return false, err
		}

		if len(submissions) >= resourceAmount {
			return false, nil
		}
	} else {
		return false, fmt.Errorf("invalid resource")
	}

	return true, nil
}

// UserCanCommentAction accepts user that has all of requiredRoles and owns given resource(s)
func (a *App) UserCanCommentAction(r *http.Request, uid int64) (bool, error) {
	if err := r.ParseForm(); err != nil {
		return false, err
	}

	ctx := r.Context()

	getUserRoles := func() (interface{}, error) {
		return a.Service.GetUserRoles(ctx, uid)
	}

	userRoles, err, cached := a.authMiddlewareCache.Memoize(fmt.Sprintf("getUserRoles-%d", uid), getUserRoles)
	if err != nil {
		return false, err
	}

	utils.LogCtx(ctx).WithField("cached", utils.BoolToString(cached))

	formAction := r.FormValue("action")

	canDo := func(actions, roles []string) bool {
		for _, action := range actions {
			if action == formAction {
				for _, userRole := range userRoles.([]string) {
					hasRole := false
					for _, role := range roles {
						if role == userRole {
							hasRole = true
							break
						}
					}
					if hasRole {
						return true
					}
				}
				break
			}
		}
		return false
	}

	canComment := formAction == constants.ActionComment
	isAdder := canDo([]string{constants.ActionMarkAdded}, constants.AdderRoles())
	isDecider := canDo([]string{constants.ActionApprove, constants.ActionRequestChanges,
		constants.ActionVerify, constants.ActionAssignTesting, constants.ActionUnassignTesting,
		constants.ActionAssignVerification, constants.ActionUnassignVerification, constants.ActionReject}, constants.DeciderRoles())

	return canComment || isAdder || isDecider, nil
}

func muxAny(authorizers ...func(*http.Request, int64) (bool, error)) func(*http.Request, int64) (bool, error) {
	return func(r *http.Request, uid int64) (bool, error) {
		for _, authorizer := range authorizers {
			ok, err := authorizer(r, uid)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	}
}

func muxAll(authorizers ...func(*http.Request, int64) (bool, error)) func(*http.Request, int64) (bool, error) {
	return func(r *http.Request, uid int64) (bool, error) {
		isAuthorized := true
		for _, authorizer := range authorizers {
			ok, err := authorizer(r, uid)
			if err != nil {
				return false, err
			}
			if !ok {
				isAuthorized = false
				break
			}
		}

		if !isAuthorized {
			return false, nil
		}
		return true, nil
	}
}
