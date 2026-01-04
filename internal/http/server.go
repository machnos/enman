package http

import (
	"context"
	"crypto/rand"
	"embed"
	"enman/internal/config"
	"enman/internal/domain"
	"enman/internal/domain/repository"
	"enman/internal/http/api"
	"enman/internal/http/api/battery"
	"enman/internal/http/api/electricity"
	"enman/internal/http/api/gas"
	"enman/internal/http/api/prices"
	"enman/internal/log"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/sessions"
)

const (
	d3           = "d3/d3-7.9.0.min.js"
	jquery       = "jquery/jquery-3.6.4.min.js"
	plot         = "plot/plot-0.6.17.min.js"
	bootstrapJs  = "bootstrap/bootstrap.bundle-5.3.3.min.js"
	bootstrapCss = "bootstrap/bootstrap-5.3.3.min.css"
)

//go:embed template/*
//go:embed static/*
var staticContent embed.FS
var templates *template.Template

// generateRandomSecret generates a cryptographically secure random secret
func generateRandomSecret(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	// Convert to hex string for better compatibility
	return fmt.Sprintf("%x", bytes), nil
}

type Server struct {
	system      *domain.System
	repository  repository.Repository
	server      *http.Server
	contextRoot string
	cookieStore *sessions.CookieStore
}

func NewServer(config *config.Http, system *domain.System, repository repository.Repository) (*Server, error) {
	// Get or generate session secret
	sessionSecret := config.SessionSecret
	if sessionSecret == "" {
		// Generate a secure random secret if not provided in configuration
		secret, err := generateRandomSecret(32)
		if err != nil {
			return nil, fmt.Errorf("failed to generate session secret: %w", err)
		}
		sessionSecret = secret
		log.Warning("No session secret provided in configuration. Generated a random one. For production, set 'session_secret' in http configuration.")
	}

	s := &Server{
		system:      system,
		repository:  repository,
		cookieStore: sessions.NewCookieStore([]byte(sessionSecret)),
	}

	contextRoot := config.ContextRoot
	if contextRoot == "" {
		contextRoot = "/"
	} else {
		if !strings.HasPrefix(contextRoot, "/") {
			contextRoot = "/" + contextRoot
		}
		if len(contextRoot) > 1 && strings.HasSuffix(contextRoot, "/") {
			contextRoot = contextRoot[0 : len(contextRoot)-1]
		}
	}
	s.contextRoot = contextRoot

	r := chi.NewRouter()
	r.Use(middleware.CleanPath)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	var allFiles []string
	files, err := staticContent.ReadDir("template")
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		filename := file.Name()
		allFiles = append(allFiles, "template/"+filename)
	}
	templates, err = template.ParseFS(staticContent, allFiles...)
	if err != nil {
		return nil, err
	}

	guiPath := "gui"
	enmanMiddleware := NewMiddleware(s.cookieStore, s.mergePaths(contextRoot, guiPath))

	r.Route(contextRoot, func(r chi.Router) {
		r.Use(middleware.Compress(5))
		r.Get("/", func(writer http.ResponseWriter, request *http.Request) {
			http.Redirect(writer, request, s.mergePaths(contextRoot, guiPath), http.StatusFound)
		})
		r.Route("/"+guiPath, func(r chi.Router) {
			r.Use(enmanMiddleware.LoggedIn)
			r.Get("/", s.dashboard)
			r.Get("/login", s.login)
			r.Post("/login", s.login)
			r.Get("/logout", s.logout)
		})
		r.Get("/static/*", s.staticResource)
		r.Route("/api", api.NewBaseApi(system).Router(map[string]func(r chi.Router){
			"/electricity": electricity.NewApi(system, repository).Router(nil),
			"/battery":     battery.NewApi(system, repository).Router(nil),
			"/gas":         gas.NewApi(system, repository).Router(nil),
			"/prices":      prices.NewApi(system, repository).Router(nil),
		}))
	})

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: r,
	}
	return s, nil
}

func (s *Server) mergePaths(root string, path string) string {
	merged := root
	if !strings.HasSuffix(merged, "/") {
		merged = merged + "/"
	}
	if strings.HasPrefix(path, "/") {
		merged += path[1:]
	} else {
		merged += path
	}
	return merged
}

func (s *Server) Start() error {
	log.Infof("Starting http server at %v", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Info("Shutting down http server")
	return s.server.Shutdown(ctx)
}

func (s *Server) staticResource(w http.ResponseWriter, r *http.Request) {
	resource := chi.URLParam(r, "*")
	file, err := staticContent.ReadFile("static/" + resource)
	if err != nil {
		return
	}
	if strings.HasSuffix(resource, ".js") {
		w.Header().Set("Content-Type", "text/javascript;charset=utf-8")
	} else if strings.HasSuffix(resource, ".css") {
		w.Header().Set("Content-Type", "text/css;charset=utf-8")
	}
	_, _ = w.Write(file)
}

func (s *Server) dashboard(w http.ResponseWriter, _ *http.Request) {
	t := templates.Lookup("header.tmpl.html")
	_ = t.ExecuteTemplate(w, "header", newTemplateData(s.contextRoot).
		withStylesheets(bootstrapCss).
		withScripts(jquery, d3, plot).
		withBodyClass("d-flex").
		data)

	t = templates.Lookup("dashboard.tmpl.html")
	_ = t.ExecuteTemplate(w, "dashboard", newTemplateData(s.contextRoot).
		data)

	t = templates.Lookup("footer.tmpl.html")
	_ = t.ExecuteTemplate(w, "footer", newTemplateData(s.contextRoot).
		withScripts(bootstrapJs).
		data)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	loginData := newTemplateData(s.contextRoot)
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		username := r.PostForm.Get("username")
		password := r.PostForm.Get("password")
		if username == "" {
			loginData.addError("username", "username must not be empty")
		}
		if password == "" {
			loginData.addError("password", "password must not be empty")
		}
		// TODO authenticate
		if !loginData.hasError() {
			session, err := s.cookieStore.Get(r, "session.id")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			session.Values["username"] = username
			session.Options.MaxAge = 0
			session.Options.HttpOnly = true
			err = session.Save(r, w)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			redirect := r.URL.Query().Get("redirect")
			if redirect == "" {
				redirect = s.contextRoot
			}
			http.Redirect(w, r, redirect, http.StatusFound)
			return
		}
	}
	t := templates.Lookup("header.tmpl.html")
	_ = t.ExecuteTemplate(w, "header", newTemplateData(s.contextRoot).
		withStylesheets(bootstrapCss).
		withBodyClass("d-flex align-items-center").
		data)

	t = templates.Lookup("login.tmpl.html")
	_ = t.ExecuteTemplate(w, "login", loginData)

	t = templates.Lookup("footer.tmpl.html")
	_ = t.ExecuteTemplate(w, "footer", newTemplateData(s.contextRoot).
		withScripts(bootstrapJs).
		data)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	session, err := s.cookieStore.Get(r, "session.id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	session.Options.MaxAge = -1
	err = session.Save(r, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, s.contextRoot, http.StatusFound)
}
