package ui

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

//go:embed assets/*
var assets embed.FS

var names = map[string]string{
	"swagger": "assets/swagger.html",
	"scalar":  "assets/scalar.html",
	"redoc":   "assets/redoc.html",
}

type pageData struct {
	SpecURL string
}

func Handler(name, specURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		html, err := Render(name, specURL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	}
}

func AssetsHandler() http.Handler {
	return http.FileServer(http.FS(assets))
}

func Render(name, specURL string) (string, error) {
	path, ok := names[strings.ToLower(name)]
	if !ok {
		return "", fmt.Errorf("unknown OpenAPI UI %q", name)
	}
	data, err := assets.ReadFile(path)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New(name).Parse(string(data))
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, pageData{SpecURL: specURL}); err != nil {
		return "", err
	}
	return buf.String(), nil
}
