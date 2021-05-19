package main

import (
	"context"

	"github.com/sirupsen/logrus"
)

type contextString string

type ctxKeys struct {
	UserID    contextString
	Log       contextString
	RequestID contextString
}

// CtxKeys is context value keys
var CtxKeys = ctxKeys{
	UserID:    "userID",
	Log:       "Log",
	RequestID: "requestID",
}

// UserIDFromContext extracts userID from context
func UserIDFromContext(ctx context.Context) int64 {
	v := ctx.Value(CtxKeys.UserID)
	if v == nil {
		return 0
	}
	return v.(int64)
}

// RequestIDFromContext extracts requestID from context
func RequestIDFromContext(ctx context.Context) string {
	v := ctx.Value(CtxKeys.RequestID)
	if v == nil {
		return ""
	}
	return v.(string)
}

// LogCtx returns logger with certain context values included
func LogCtx(ctx context.Context) *logrus.Entry {
	l := ctx.Value(CtxKeys.Log).(*logrus.Logger)
	entry := logrus.NewEntry(l)

	if userID := UserIDFromContext(ctx); userID != 0 {
		entry = entry.WithField(string(CtxKeys.UserID), userID)
	}
	if requestID := RequestIDFromContext(ctx); len(requestID) > 0 {
		entry = entry.WithField(string(CtxKeys.RequestID), requestID)
	}

	return entry
}
