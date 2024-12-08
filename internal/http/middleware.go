package http

import (
	"github.com/gorilla/sessions"
	"net/http"
	"net/url"
)

type Middleware struct {
	store   *sessions.CookieStore
	guiPath string
}

func NewMiddleware(cookieStore *sessions.CookieStore, guiPath string) *Middleware {
	return &Middleware{
		store:   cookieStore,
		guiPath: guiPath,
	}
}
func (m *Middleware) LoggedIn(next http.Handler) http.Handler {
	loginPath := m.guiPath + "/login"

	fn := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == loginPath {
			next.ServeHTTP(w, r)
			return
		}
		session, err := m.store.Get(r, "session.id")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		username := session.Values["username"]
		if username == nil {
			http.Redirect(w, r, loginPath+"?redirect="+url.QueryEscape(r.URL.Path), http.StatusFound)
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
