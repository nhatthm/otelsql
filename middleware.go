package otelsql

type middleware[T any] func(next T) T

// chainMiddlewares builds an inline middleware stack in the order they are passed.
func chainMiddlewares[T any](middlewares []middleware[T], last T) T {
	// Return ahead of time if there are not any middlewares for the chain.
	if len(middlewares) == 0 {
		return last
	}

	// Wrap the end execer with the middleware chain.
	h := middlewares[len(middlewares)-1](last)

	for i := len(middlewares) - 2; i >= 0; i-- {
		h = middlewares[i](h)
	}

	return h
}
