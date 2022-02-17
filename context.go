package otelsql

import "context"

type queryCtxKey struct{}

// QueryFromContext gets the query from context.
func QueryFromContext(ctx context.Context) string {
	query, ok := ctx.Value(queryCtxKey{}).(string)
	if !ok {
		return ""
	}

	return query
}

// ContextWithQuery attaches the query to the parent context.
func ContextWithQuery(ctx context.Context, query string) context.Context {
	return context.WithValue(ctx, queryCtxKey{}, query)
}
