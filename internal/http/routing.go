package http

import (
	"encoding/json"
	"enman/internal"
	"enman/internal/config"
	"enman/internal/log"
	"enman/internal/persistency"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"html/template"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	highcharts = "highcharts-10.3.3.js"
)

var templates *template.Template

type systemWrapper struct {
	system *internal.System
}

func StartServer(config *config.Http, system *internal.System, repository persistency.Repository) error {
	r := chi.NewRouter()
	r.Use(middleware.Timeout(30 * time.Second))
	s := &systemWrapper{
		system: system,
	}

	var allFiles []string
	files, err := os.ReadDir("web/template")
	if err != nil {
		return err
	}
	for _, file := range files {
		filename := file.Name()
		if strings.HasSuffix(filename, ".tmpl") {
			allFiles = append(allFiles, "web/template/"+filename)
		}
	}
	templates, err = template.ParseFiles(allFiles...)

	r.Get("/static/*", s.staticResource)
	r.Get("/", s.dashboard)
	r.Get("/api", s.dataAsJson)
	log.Infof("Starting http server at port %d", config.Port)
	err = http.ListenAndServe(fmt.Sprintf(":%d", config.Port), r)
	if err != nil {
		return err
	}
	return nil
}

func (s *systemWrapper) staticResource(w http.ResponseWriter, r *http.Request) {
	resource := chi.URLParam(r, "*")
	file, err := os.ReadFile("web/static/" + resource)
	if err != nil {
		return
	}
	w.Write(file)
}

func (s *systemWrapper) dashboard(w http.ResponseWriter, r *http.Request) {
	data := newAppData([]string{highcharts})
	t := templates.Lookup("header.tmpl")
	t.ExecuteTemplate(w, "header", data)

	t = templates.Lookup("dashboard.tmpl")
	t.ExecuteTemplate(w, "dashboard", nil)

	t = templates.Lookup("footer.tmpl")
	t.ExecuteTemplate(w, "footer", nil)
}

func (s *systemWrapper) dataAsJson(w http.ResponseWriter, r *http.Request) {
	data, err := json.Marshal(map[string]any{
		"system": s.system.ToMap(),
	})
	if err != nil {
		return
	}
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	_, _ = w.Write(data)
	//g := *h.system.Grid()
	//_, _ = io.WriteString(w, fmt.Sprintf("Phases: %d, Power %4.2fW (L1: %4.2fW, L2: %4.2fW, L3: %4.2fW), Current %4.2fA (L1: %4.2fA, L2: %4.2fA, L3: %4.2fA), Voltage (L1: %4.2fV, L2: %4.2fV, L3: %4.2fV)",
	//	g.Phases(),
	//	g.TotalPower(), g.Power(0), g.Power(1), g.Power(2),
	//	g.TotalCurrent(), g.Current(0), g.Current(1), g.Current(2),
	//	g.Voltage(0), g.Voltage(1), g.Voltage(2)))
}

func newAppData(scripts []string) map[string]any {
	return map[string]any{
		"ContextRoot": "/",
		"Scripts":     scripts,
	}
}
