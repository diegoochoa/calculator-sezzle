package httpx

import "context"

type contextKey int

const (
	requestIDKey contextKey = iota
	subjectKey
)

// WithRequestID returns a context carrying the id assigned to this request.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestID returns the request id, or "" outside a request.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// WithSubject returns a context carrying the authenticated client id.
func WithSubject(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, subjectKey, subject)
}

// Subject returns the authenticated client id, or "" when unauthenticated.
func Subject(ctx context.Context) string {
	subject, _ := ctx.Value(subjectKey).(string)
	return subject
}
