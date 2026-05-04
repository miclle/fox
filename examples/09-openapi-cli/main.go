package main

import (
	"os"

	"github.com/fox-gonic/fox/examples/09-openapi-cli/internal/server"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	engine := server.NewEngine()
	engine.Run(":" + port)
}
