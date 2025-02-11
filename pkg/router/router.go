package router

import (
	"context"
	"net/http"
	"time"

	ghastContainer "github.com/bradcypert/ghast/pkg/container"
	pathToRegexp "github.com/soongo/path-to-regexp"
)

const (
	connect string = "CONNECT"
	delete  string = "DELETE"
	get     string = "GET"
	head    string = "HEAD"
	options string = "OPTIONS"
	patch   string = "PATCH"
	post    string = "POST"
	put     string = "PUT"
	trace   string = "TRACE"
)

// MiddlewareFunc is a functional alias to signify the signature of a middleware anonymous function.
type MiddlewareFunc = func(*http.ResponseWriter, *http.Request)

// Binding maps a url pattern to an HTTP handler
type Binding struct {
	handler     func(http.ResponseWriter, *http.Request)
	match       string
	method      string
	middlewares []MiddlewareFunc
}

// Router struct models our routes to match off of and their behavior
type Router struct {
	path        string
	binding     []Binding
	container   *ghastContainer.Container
	middlewares []MiddlewareFunc
}

// Base set the basepath for this router segment
func (r *Router) Base(prefix string) *Router {
	r.path = prefix
	return r
}

// Merge merges one or more routers into the calling Router
func (r *Router) Merge(routers ...*Router) *Router {
	for _, router := range routers {
		for _, binding := range router.binding {
			// generate full binding for router
			binding.match = router.path + binding.match

			// merge router specific middlewares into route specific middlewares
			binding.middlewares = append(binding.middlewares, router.middlewares...)

			// append full binding the base router
			r.binding = append(r.binding, binding)
		}
	}

	return r
}

// AddMiddleware registers a new global middleware within the router
// global middlewares will be called on any route that matches a route binding
func (r *Router) AddMiddleware(fn []MiddlewareFunc) *Router {
	r.middlewares = append(r.middlewares, fn...)
	return r
}

// SetDIContainer sets up the DI container that this router will use.
// The provided DI container will be available in all controllers via context (or controller helpers)
func (r *Router) SetDIContainer(c *ghastContainer.Container) {
	r.container = c
}

// ServeHTTP allows the router to adhere to the handler func requirements
// Delegates requests to provided routes
func (r Router) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	for _, binding := range r.binding {
		if req.Method == binding.method {
			var tokens []pathToRegexp.Token
			regexp := pathToRegexp.Must(pathToRegexp.PathToRegexp(r.path+binding.match, &tokens, nil))

			match, _ := regexp.FindStringMatch(req.URL.String())

			if match != nil {

				// Run global middlewares
				for _, m := range r.middlewares {
					m(&rw, req)
				}

				// Run route specific middlewares
				for _, m := range binding.middlewares {
					m(&rw, req)
				}

				ctx := req.Context()
				for _, g := range match.Groups() {
					if len(tokens) >= match.Index && len(tokens) > 0 {
						key := tokens[match.Index].Name
						value := g.String()
						ctx = context.WithValue(ctx, key, value)
					}
				}

				r := req.WithContext(ctx)
				binding.handler(rw, r)
				break
			}
		}
	}
}

// PathParam Get a Path Parameter from a given request and key
func (r *Router) PathParam(req *http.Request, key string) interface{} {
	return req.Context().Value(key)
}

// PathParam Get a Path Parameter from a given request and key
func (r *Router) QueryParam(req *http.Request, key string) interface{} {
	return req.URL.Query()[key]
}

// Get registers a new GET route with the router
func (r *Router) Get(route string, f http.HandlerFunc, middleware ...MiddlewareFunc) *Router {
	return r.route(get, route, f, middleware)
}

// Post registers a new POST route with the router
func (r *Router) Post(route string, f http.HandlerFunc, middleware ...MiddlewareFunc) *Router {
	return r.route(post, route, f, middleware)
}

// Put registeres a new PUT route with the router
func (r *Router) Put(route string, f http.HandlerFunc, middleware ...MiddlewareFunc) *Router {
	return r.route(put, route, f, middleware)
}

// Patch registers a new PATCH route with the router
func (r *Router) Patch(route string, f http.HandlerFunc, middleware ...MiddlewareFunc) *Router {
	return r.route(patch, route, f, middleware)
}

// Delete registers a new DELETE route with the router
func (r *Router) Delete(route string, f http.HandlerFunc, middleware ...MiddlewareFunc) *Router {
	return r.route(delete, route, f, middleware)
}

// Options registers a new OPTIONS route with the router
func (r *Router) Options(route string, f http.HandlerFunc, middleware ...MiddlewareFunc) *Router {
	return r.route(options, route, f, middleware)
}

// Head registers a new HEAD route with the router
func (r *Router) Head(route string, f http.HandlerFunc, middleware ...MiddlewareFunc) *Router {
	return r.route(head, route, f, middleware)
}

// Trace registers a new TRACE route with the router
func (r *Router) Trace(route string, f http.HandlerFunc, middleware ...MiddlewareFunc) *Router {
	return r.route(trace, route, f, middleware)
}

// Connect registers a new CONNECT route with the router
func (r *Router) Connect(route string, f http.HandlerFunc, middleware ...MiddlewareFunc) *Router {
	return r.route(connect, route, f, middleware)
}

// Resource takes in a Controller Resource and generates Index, Get, Create, Delete
// and Update routes for that resource.
// Prefix is a string for any scoping or namespacing for the resource
// For example, you can pass "/v1/" as a Prefix for a Resource where
// getName returns "user" which would create the following routes:
// GET     /v1/user
// GET     /v1/user/:id
// POST    /v1/user
// DELETE  /v1/user/:id
// UPDATE /v1/user/:id
func (r *Router) Resource(prefix string, resource Resource) {
	r.Get(prefix+resource.GetName(), resource.Index)
	r.Get(prefix+resource.GetName()+"/:id", resource.Get)
	r.Post(prefix+resource.GetName(), resource.Create)
	r.Delete(prefix+resource.GetName()+"/:id", resource.Delete)
	r.Put(prefix+resource.GetName()+"/:id", resource.Update)
}

// DefaultServer is an optional method to help get a preconfigured server
// with the router bound as the handler and some sensible defaults
func (r Router) DefaultServer() *http.Server {
	return &http.Server{
		Addr:           ":9000",
		Handler:        r,
		MaxHeaderBytes: 1 << 20,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
	}
}

func (r *Router) route(method string, route string, f http.HandlerFunc, middleware []MiddlewareFunc) *Router {
	r.binding = append(r.binding, Binding{match: route, handler: f, middlewares: middleware, method: method})
	return r
}
