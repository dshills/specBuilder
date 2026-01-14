package main

import (
	"context"
	_ "embed"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dshills/specbuilder/backend/internal/api"
	"github.com/dshills/specbuilder/backend/internal/compiler"
	"github.com/dshills/specbuilder/backend/internal/llm"
	"github.com/dshills/specbuilder/backend/internal/repository/sqlite"
	"github.com/dshills/specbuilder/backend/internal/validator"
)

//go:embed schemas/ProjectImplementationSpec.schema.json
var specSchemaJSON string

func loadSpecSchema() (string, error) {
	return specSchemaJSON, nil
}

func logConfig() {
	log.Println("=== SpecBuilder Configuration ===")

	// Log SPECBUILDER_* env vars
	envVars := []struct {
		name         string
		defaultValue string
	}{
		{"SPECBUILDER_API_PORT", "8080"},
		{"SPECBUILDER_DB_PATH", "data/specbuilder.db"},
		{"SPECBUILDER_CORS_ORIGINS", "* (allow all)"},
		{"SPECBUILDER_LLM_PROVIDER", "(auto-detect)"},
		{"SPECBUILDER_LLM_MODEL", "(auto-detect)"},
	}

	for _, ev := range envVars {
		value := os.Getenv(ev.name)
		if value == "" {
			log.Printf("  %s: %s (default)", ev.name, ev.defaultValue)
		} else {
			log.Printf("  %s: %s", ev.name, value)
		}
	}

	// Log API key availability (not the actual keys)
	apiKeys := []string{"ANTHROPIC_API_KEY", "GEMINI_API_KEY", "OPENAI_API_KEY"}
	var configured []string
	for _, key := range apiKeys {
		if os.Getenv(key) != "" {
			configured = append(configured, key)
		}
	}
	if len(configured) > 0 {
		log.Printf("  API keys configured: %v", configured)
	} else {
		log.Println("  API keys configured: (none)")
	}

	log.Println("=================================")
}

func main() {
	logConfig()

	port := os.Getenv("SPECBUILDER_API_PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("SPECBUILDER_DB_PATH")
	if dbPath == "" {
		// Default to data directory in project root
		dbPath = filepath.Join("data", "specbuilder.db")
	}

	// Ensure data directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Initialize repository
	repo, err := sqlite.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer repo.Close()

	// Initialize validator
	val, err := validator.New()
	if err != nil {
		log.Fatalf("Failed to initialize validator: %v", err)
	}

	// Initialize LLM factory (optional - server works without it for basic CRUD)
	// Supports: GEMINI_API_KEY (preferred), OPENAI_API_KEY (fallback)
	var compilerSvc *compiler.Service

	llmFactory := llm.NewFactory()
	if llmFactory.Available() {
		// Load spec schema for compiler
		specSchema, err := loadSpecSchema()
		if err != nil {
			log.Fatalf("Failed to load spec schema: %v", err)
		}
		compilerSvc = compiler.NewService(llmFactory, val, specSchema)
		log.Printf("LLM factory initialized (default: %s/%s)", llmFactory.DefaultProvider(), llmFactory.DefaultModel())
	} else {
		log.Println("Warning: No LLM API key set (GEMINI_API_KEY or OPENAI_API_KEY) - compilation endpoints will be disabled")
	}

	// Initialize API handler
	handler := api.NewHandler(repo, compilerSvc)

	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Register API routes
	handler.RegisterRoutes(mux)

	// Apply middleware
	var h http.Handler = mux
	h = api.Logger(h)
	corsOrigins := os.Getenv("SPECBUILDER_CORS_ORIGINS")
	h = api.CORS(api.CORSConfig{AllowedOrigins: corsOrigins})(h)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      h,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second, // Longer for compilation
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		log.Println("Shutting down server...")
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	log.Printf("Server starting on port %s", port)
	log.Printf("Database: %s", dbPath)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
	log.Println("Server stopped")
}
