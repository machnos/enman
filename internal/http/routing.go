package http

import (
	"context"
	"embed"
	"enman/internal/config"
	"enman/internal/domain"
	"enman/internal/http/api"
	"enman/internal/http/api/battery"
	"enman/internal/http/api/electricity"
	"enman/internal/http/api/prices"
	"enman/internal/log"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"html/template"
	"net/http"
	"strings"
	"time"
)

const (
	d3     = "d3/d3-7.8.5.min.js"
	jquery = "jquery/jquery-3.6.4.min.js"
	plot   = "plot/plot-0.6.10.min.js"
)

//go:embed template/*
//go:embed static/*
var staticContent embed.FS
var templates *template.Template

type Server struct {
	system     *domain.System
	repository domain.Repository
	server     *http.Server
}

func NewServer(config *config.Http, system *domain.System, repository domain.Repository) (*Server, error) {
	s := &Server{
		system:     system,
		repository: repository,
	}

	contextRoot := config.ContextRoot
	if contextRoot == "" {
		contextRoot = "/"
	} else {
		if !strings.HasPrefix(contextRoot, "/") {
			contextRoot = "/" + contextRoot
		}
		if strings.HasSuffix(contextRoot, "/") {
			contextRoot = contextRoot[0 : len(contextRoot)-1]
		}

	}

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

	r.Route(contextRoot, func(r chi.Router) {
		r.Get("/", s.dashboard)
		r.Get("/static/*", s.staticResource)
		r.Route("/api", api.NewBaseApi(system, repository).Router(map[string]func(r chi.Router){
			"/electricity": electricity.NewElectricityApi(system, repository).Router(nil),
			"/battery":     battery.NewBatteryApi(system, repository).Router(nil),
			"/prices":      prices.NewPricesApi(system, repository).Router(nil),
		}))
	})

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: r,
	}
	return s, nil
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
	}
	_, _ = w.Write(file)
}

func (s *Server) dashboard(w http.ResponseWriter, req *http.Request) {
	data := newAppData(req.URL.Path, []string{jquery, d3, plot})

	t := templates.Lookup("header.tmpl.html")
	_ = t.ExecuteTemplate(w, "header", data)

	t = templates.Lookup("dashboard.tmpl.html")
	_ = t.ExecuteTemplate(w, "dashboard", data)

	t = templates.Lookup("footer.tmpl.html")
	_ = t.ExecuteTemplate(w, "footer", nil)
}

func newAppData(contextRoot string, scripts []string) map[string]any {
	return map[string]any{
		"ContextRoot": contextRoot,
		"Scripts":     scripts,
	}
}
