package foundationhttp

import (
	"net/http"
	"slices"
)

// Middleware is the signature of a Middleware function that is used
// the Router
type Middleware func(http.Handler) http.Handler

// Router defines a custom type that embeds http.ServeMux and a slice
// of middlewares to be applied globally to all routes
type Router struct {
	mux     *http.ServeMux
	globals []Middleware
}

// New instantiates a new Router with a mux constructed
func (r *Router) New() *Router {
	return &Router{
		mux: http.NewServeMux(),
	}
}

// Use adds  middleware functions to be appliced
// to specific routes after the global ones have been appliedj
func (r *Router) Use(mids ...Middleware) {
	r.globals = append(r.globals, mids...)
}

// Handle will make sure th handle function is wrapped in all middleware functions
// that was received as param
func (r *Router) Handle(pattern string, handler http.HandlerFunc, mws ...Middleware) {

	// h will be wrap in additiona middleware and invoked
	var h http.Handler = handler

	// apply the middleware stack in a reverse so the first function in the
	// middleware stack gets called first. The last in the slice is the
	// closest to the handler
	for _, m := range slices.Backward(mws) {
		if m != nil {
			h = m(h)
		}
	}

	r.mux.Handle(pattern, h)
}

// Build applies to the Router's embeded ServeMux all the functions in
// globals sliced
func (r *Router) Build() http.Handler {

	var h http.Handler = r.mux

	for _, m := range slices.Backward(r.globals) {
		if m != nil {
			h = m(h)
		}
	}

	return r.mux
}
