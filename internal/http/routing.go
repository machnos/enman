package http

import (
	"embed"
	"enman/internal"
	"enman/internal/config"
	"enman/internal/http/api"
	"enman/internal/http/api/energy_flow"
	"enman/internal/http/api/prices"
	"enman/internal/log"
	"enman/internal/persistency"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"html/template"
	"net/http"
	"strings"
	"time"
)

const (
	d3                      = "d3/d3-7.8.4.min.js"
	highcharts              = "highcharts-10.3.3/highcharts.js"
	highchartsAccessibility = "highcharts-10.3.3/accessibility.js"
	highchartsMore          = "highcharts-10.3.3/highcharts-more.js"
	highchartsSolidGauge    = "highcharts-10.3.3/solid-gauge.js"
	htl                     = "htl/htl-0.3.1.min.js"
	jquery                  = "jquery/jquery-3.6.4.min.js"
	moment                  = "moment-with-locales.min.js"
	momentWithData          = "moment-timezone-with-data.min.js"
	plot                    = "plot/plot-0.6.5.min.js"
)

//go:embed template/*
//go:embed static/*
var staticContent embed.FS
var templates *template.Template

type systemWrapper struct {
	system     *internal.System
	repository persistency.Repository
}

func StartServer(config *config.Http, system *internal.System, repository persistency.Repository) error {
	r := chi.NewRouter()
	r.Use(middleware.CleanPath)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	s := &systemWrapper{
		system:     system,
		repository: repository,
	}

	var allFiles []string
	files, err := staticContent.ReadDir("template")
	if err != nil {
		return err
	}
	for _, file := range files {
		filename := file.Name()
		allFiles = append(allFiles, "template/"+filename)
	}
	templates, err = template.ParseFS(staticContent, allFiles...)
	if err != nil {
		return err
	}
	r.Get("/static/*", s.staticResource)
	r.Get("/", s.dashboard)

	// Setup api endpoints
	rootApiRouter := api.NewBaseApi(system, repository).Router(map[string]func(r chi.Router){
		"/energy_flow": energy_flow.NewEnergyFlowApi(system, repository).Router(nil),
		"/prices":      prices.NewPricesApi(system, repository).Router(nil),
	})
	r.Route("/api", rootApiRouter)
	log.Infof("Starting http server at port %d", config.Port)
	err = http.ListenAndServe(fmt.Sprintf(":%d", config.Port), r)
	if err != nil {
		return err
	}
	return nil
}

func (s *systemWrapper) staticResource(w http.ResponseWriter, r *http.Request) {
	resource := chi.URLParam(r, "*")
	file, err := staticContent.ReadFile("static/" + resource)
	if err != nil {
		return
	}
	if strings.HasSuffix(resource, ".js") {
		w.Header().Set("Content-Type", "text/javascript;charset=utf-8")
	}
	w.Write(file)
}

func (s *systemWrapper) dashboard(w http.ResponseWriter, r *http.Request) {
	data := newAppData([]string{jquery, d3, plot, htl})
	t := templates.Lookup("header.tmpl.html")
	t.ExecuteTemplate(w, "header", data)

	t = templates.Lookup("dashboard.tmpl.html")
	t.ExecuteTemplate(w, "dashboard", nil)

	t = templates.Lookup("footer.tmpl.html")
	t.ExecuteTemplate(w, "footer", nil)
}

func newAppData(scripts []string) map[string]any {
	return map[string]any{
		"ContextRoot": "/",
		"Scripts":     scripts,
	}
}
