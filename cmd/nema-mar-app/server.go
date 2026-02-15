package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/GeoNet/kit/health"
	"github.com/GeoNet/nema-mar-portal/internal/fastschema"
)

// sourceDir returns the directory of this source file, used to resolve
// paths relative to the source tree when running locally (not in Docker).
func sourceDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filename)
}

var (
	fsClient *fastschema.Client
)

func main() {
	if health.RunningHealthCheck() {
		healthCheck()
	}

	fsURL := os.Getenv("FASTSCHEMA_URL")
	if fsURL == "" {
		fsURL = "http://localhost:8000"
	}

	fsClient = fastschema.NewClient(fsURL)

	// Authenticate with FastSchema sidecar.
	fsUser := os.Getenv("FS_ADMIN_USER")
	fsPass := os.Getenv("FS_ADMIN_PASS")
	if fsUser != "" && fsPass != "" {
		if err := fsClient.Login(fsUser, fsPass); err != nil {
			log.Printf("warning: failed to login to FastSchema: %v", err)
		}
	}

	// Apply schema on startup.
	schemaPath := os.Getenv("SCHEMA_PATH")
	if schemaPath == "" {
		schemaPath = "/app/schema/eat.json"
		if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
			// Fall back to source-relative path for local development.
			schemaPath = filepath.Join(sourceDir(), "..", "..", "schema", "eat.json")
		}
	}
	schemaJSON, err := os.ReadFile(schemaPath)
	if err != nil {
		log.Printf("warning: could not read schema file %s: %v", schemaPath, err)
	} else {
		if err := fsClient.ApplySchema(schemaJSON); err != nil {
			log.Printf("warning: failed to apply schema: %v", err)
		}
	}

	// Load templates.
	templateDir := os.Getenv("TEMPLATE_DIR")
	if templateDir == "" {
		templateDir = "/app/templates"
		if _, err := os.Stat(templateDir); os.IsNotExist(err) {
			// Fall back to source-relative path for local development.
			templateDir = filepath.Join(sourceDir(), "templates")
		}
	}
	if err := loadTemplates(templateDir); err != nil {
		log.Fatalf("failed to load templates: %v", err)
	}

	log.Println("starting server")
	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  1 * time.Minute,
		WriteTimeout: 10 * time.Minute,
	}
	log.Fatal(server.ListenAndServe())
}

func healthCheck() {
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	msg, err := health.Check(ctx, ":8080/soh", timeout)
	if err != nil {
		log.Printf("status: %v", err)
		os.Exit(1)
	}
	log.Printf("status: %s", string(msg))
	os.Exit(0)
}
