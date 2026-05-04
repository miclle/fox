package ui

import (
	"strings"
	"testing"
)

func TestRenderOfflinePages(t *testing.T) {
	for _, name := range []string{"swagger", "scalar", "redoc"} {
		html, err := Render(name, "/openapi.yaml")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.Contains(html, "/openapi.yaml") {
			t.Fatalf("%s page missing spec URL", name)
		}
		if strings.Contains(html, "https://") || strings.Contains(html, "http://") || strings.Contains(html, "//cdn") {
			t.Fatalf("%s page contains external URL:\n%s", name, html)
		}
	}
}
