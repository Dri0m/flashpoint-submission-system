package transport

import (
	"context"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/gorilla/mux"
	"net/http"
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

// TODO optimize database access in middleware

// UserAuthMux takes many authorization middlewares and accepts if any of them does not return error
func (a *App) UserAuthMux(next func(http.ResponseWriter, *http.Request), authorizers ...func(*http.Request, int64) (bool, error)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		secret, err := a.GetSecretFromCookie(r)

		if err != nil {
			utils.LogCtx(ctx).Error(err)
			utils.UnsetCookie(w, utils.Cookies.Login)
			http.Redirect(w, r, "/", http.StatusFound)
			// TODO add user facing error message here
			//http.Error(w, "failed to parse cookie, please clear your cookies and try again", http.StatusBadRequest)
			return
		}

		uid, ok, err := a.Service.GetUIDFromSession(ctx, secret)
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			utils.UnsetCookie(w, utils.Cookies.Login)
			http.Redirect(w, r, "/", http.StatusFound)
			// TODO add user facing error message here
			//http.Error(w, "failed to load session, please clear your cookies and try again", http.StatusBadRequest)
			return
		}
		if !ok {
			utils.LogCtx(ctx).Error(err)
			utils.UnsetCookie(w, utils.Cookies.Login)
			http.Redirect(w, r, "/", http.StatusFound)
			// TODO add user facing error message here
			//http.Error(w, "session expired, please log in to continue", http.StatusUnauthorized)
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
		return
	}
}

// UserHasAllRoles accepts user that has at least all requiredRoles
func (a *App) UserHasAllRoles(ctx context.Context, uid int64, requiredRoles []string) (bool, error) {
	userRoles, err := a.Service.GetUserRoles(ctx, uid)
	if err != nil {
		return false, fmt.Errorf("failed to get user roles")
	}

	isAuthorized := true

	for _, role := range userRoles {
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
	userRoles, err := a.Service.GetUserRoles(r.Context(), uid)
	if err != nil {
		return false, err
	}

	isAuthorized := constants.HasAnyRole(userRoles, roles)
	if !isAuthorized {
		return false, nil
	}

	return true, nil
}

// UserOwnsResource accepts user that owns given resource(s)
func (a *App) UserOwnsResource(r *http.Request, uid int64, resourceKey string) (bool, error) {
	ctx := r.Context()

	if resourceKey == constants.ResourceKeySubmissionID {
		params := mux.Vars(r)
		submissionID := params[constants.ResourceKeySubmissionID]
		sid, err := strconv.ParseInt(submissionID, 10, 64)
		if err != nil {
			return false, fmt.Errorf("invalid submission id")
		}

		submissions, err := a.Service.SearchSubmissions(ctx, &types.SubmissionsFilter{SubmissionIDs: []int64{sid}})
		if err != nil {
			return false, err
		}

		if len(submissions) == 0 {
			return false, fmt.Errorf("submission with id %d not found", sid)
		}

		s := submissions[0]
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
			// TODO optimize search query
			submissions, err := a.Service.SearchSubmissions(ctx, &types.SubmissionsFilter{SubmissionIDs: []int64{sid}})
			if err != nil {
				return false, fmt.Errorf("failed to load submission with id %d", sid)
			}

			if len(submissions) == 0 {
				return false, fmt.Errorf("submission with id %d not found", sid)
			}

			submission := submissions[0]

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

		submissionFiles, err := a.Service.GetSubmissionFiles(ctx, []int64{fid})
		if err != nil {
			return false, err
		}

		sf := submissionFiles[0]
		if sf.SubmitterID != uid {
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
		submissions, err := a.Service.SearchSubmissions(ctx, &types.SubmissionsFilter{SubmitterID: &uid})
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

	userRoles, err := a.Service.GetUserRoles(r.Context(), uid)
	if err != nil {
		return false, err
	}

	formAction := r.FormValue("action")

	canDo := func(actions, roles []string) bool {
		for _, action := range actions {
			if action == formAction {
				for _, userRole := range userRoles {
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
		constants.ActionAssignVerification, constants.ActionUnassignVerification}, constants.DeciderRoles())

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
