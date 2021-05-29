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
)

// TODO optimize database access in middlewares

// UserAuthentication accepts valid session cookie
func (a *App) UserAuthentication(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := a.GetUserIDFromCookie(r)
		if err != nil {
			utils.LogCtx(r.Context()).Error(err)
			http.Error(w, "failed to load session, please clear your cookies and try again", http.StatusBadRequest)
			return
		}
		if userID == 0 {
			utils.LogCtx(r.Context()).Error(err)
			http.Error(w, "please log in to continue", http.StatusUnauthorized)
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), utils.CtxKeys.UserID, userID))
		next(w, r)
	}
}

// UserAuthorizationMuxAny takes many authorization middlewares and accepts if any of them does not return error
func (a *App) UserAuthorizationMuxAny(next func(http.ResponseWriter, *http.Request), authorizers ...func(r *http.Request, uid int64) (bool, error)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(authorizers) == 0 {
			panic("no authorizer supplied")
		}

		uid, err := a.GetUserIDFromCookie(r)
		if err != nil {
			utils.LogCtx(r.Context()).Error(err)
			http.Error(w, "failed to load session, please clear your cookies and try again", http.StatusBadRequest)
			return
		}
		if uid == 0 {
			utils.LogCtx(r.Context()).Error(err)
			http.Error(w, "please log in to continue", http.StatusUnauthorized)
			return
		}

		for _, authorizer := range authorizers {
			ok, err := authorizer(r, uid)
			if err != nil {
				utils.LogCtx(r.Context()).Error(err)
				http.Error(w, "failed to verify authority", http.StatusInternalServerError)
				return
			}
			if ok {
				r = r.WithContext(context.WithValue(r.Context(), utils.CtxKeys.UserID, uid))
				next(w, r)
				return
			}
		}
		utils.LogCtx(r.Context()).Info("unauthorized attempt")
		http.Error(w, "you do not have the proper authorization to access this page", http.StatusUnauthorized)
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
		return false, fmt.Errorf("failed to get user roles")
	}

	isAuthorized := false

	for _, userRole := range userRoles {
		for _, role := range roles {
			if userRole == role {
				isAuthorized = true
				break
			}
		}
	}

	if !isAuthorized {
		return false, nil
	}

	return true, nil
}

// UserWithAllRolesOwnsResource accepts user that has all of requiredRoles and owns given resource(s)
func (a *App) UserWithAllRolesOwnsResource(r *http.Request, uid int64, requiredRoles []string, resourceKey string) (bool, error) {
	ctx := r.Context()
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

	if resourceKey == constants.ResourceKeySubmissionID {
		params := mux.Vars(r)
		submissionID := params[constants.ResourceKeySubmissionID]
		sid, err := strconv.ParseInt(submissionID, 10, 64)
		if err != nil {
			return false, nil
		}

		submisisons, err := a.Service.DB.SearchSubmissions(ctx, nil, &types.SubmissionsFilter{SubmissionID: &sid})
		if err != nil {
			return false, err
		}

		s := submisisons[0]
		if s.SubmitterID != uid {
			return false, nil
		}
	} else if resourceKey == constants.ResourceKeyFileID {
		params := mux.Vars(r)
		submissionID := params[constants.ResourceKeyFileID]
		fid, err := strconv.ParseInt(submissionID, 10, 64)
		if err != nil {
			return false, nil
		}

		submissionFiles, err := a.Service.DB.GetSubmissionFiles(ctx, nil, []int64{fid})
		if err != nil {
			return false, err
		}

		sf := submissionFiles[0]
		if sf.SubmitterID != uid {
			return false, nil
		}
	}

	return true, nil
}
