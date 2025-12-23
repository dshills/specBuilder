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

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
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

	// Initialize LLM client (optional - server works without it for basic CRUD)
	var compilerSvc *compiler.Service
	openAIKey := os.Getenv("OPENAI_API_KEY")
	if openAIKey != "" {
		openAIModel := os.Getenv("OPENAI_MODEL")
		if openAIModel == "" {
			openAIModel = "gpt-4o" // Default model
		}

		llmClient := llm.NewOpenAIClient(openAIKey, openAIModel)

		// Load spec schema for compiler
		specSchema, err := loadSpecSchema()
		if err != nil {
			log.Fatalf("Failed to load spec schema: %v", err)
		}

		compilerSvc = compiler.NewService(llmClient, val, specSchema)
		log.Printf("LLM client initialized (model: %s)", openAIModel)
	} else {
		log.Println("Warning: OPENAI_API_KEY not set - compilation endpoints will be disabled")
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
	h = api.CORS(h)

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
