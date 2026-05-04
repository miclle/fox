package cli

import (
	"context"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fox-gonic/fox-openapi/internal/cli/ui"
)

type ServeConfig struct {
	Addr  string
	UIs   []string
	Watch bool
	Open  bool
}

type servedSpec struct {
	mu   sync.RWMutex
	yaml []byte
	json []byte
	err  string
}

func Serve(cfg Config, serveCfg ServeConfig) error {
	state := &servedSpec{}
	if err := refreshSpec(cfg, state); err != nil {
		return err
	}
	if serveCfg.Watch {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go watchAndRefresh(ctx, cfg, state)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.yaml", state.handleYAML)
	mux.HandleFunc("/openapi.json", state.handleJSON)
	mux.HandleFunc("/docs", ui.Handler("swagger", "/openapi.yaml"))
	mux.HandleFunc("/scalar", ui.Handler("scalar", "/openapi.yaml"))
	mux.HandleFunc("/redoc", ui.Handler("redoc", "/openapi.yaml"))
	mux.Handle("/assets/", ui.AssetsHandler())
	if serveCfg.Open {
		go func() {
			time.Sleep(300 * time.Millisecond)
			_ = openBrowser("http://" + serveCfg.Addr + "/docs")
		}()
	}
	return http.ListenAndServe(serveCfg.Addr, mux)
}

func refreshSpec(cfg Config, state *servedSpec) error {
	cfg.Format = "yaml"
	yamlBytes, _, err := RunPipeline(cfg)
	if err != nil {
		state.setError(err.Error())
		return err
	}
	cfg.Format = "json"
	jsonBytes, _, err := RunPipeline(cfg)
	if err != nil {
		state.setError(err.Error())
		return err
	}
	state.set(yamlBytes, jsonBytes)
	return nil
}

func (s *servedSpec) set(yamlBytes, jsonBytes []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.yaml = append([]byte(nil), yamlBytes...)
	s.json = append([]byte(nil), jsonBytes...)
	s.err = ""
}

func (s *servedSpec) setError(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = value
}

func (s *servedSpec) handleYAML(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	_, _ = w.Write(s.yaml)
}

func (s *servedSpec) handleJSON(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(s.json)
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func normalizeUIs(values []string) []string {
	if len(values) == 0 {
		return []string{"swagger"}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
