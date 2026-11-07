package httpapi

import (
	"context"

	"github.com/awbuilds/drovik/backend/internal/storage"
)

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	userKey      contextKey = "user"
	sessionKey   contextKey = "session"
)

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func withAuth(ctx context.Context, user *storage.User, session *storage.Session) context.Context {
	ctx = context.WithValue(ctx, userKey, user)
	ctx = context.WithValue(ctx, sessionKey, session)
	return ctx
}

func CurrentUser(ctx context.Context) *storage.User {
	user, _ := ctx.Value(userKey).(*storage.User)
	return user
}

func CurrentSession(ctx context.Context) *storage.Session {
	session, _ := ctx.Value(sessionKey).(*storage.Session)
	return session
}
