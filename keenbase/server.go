package keenbase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// serve builds the router, starts the HTTP server, and blocks until a
// shutdown signal (SIGINT / SIGTERM) is received or a fatal error occurs.
func (sb *SimpleBase) serve() error {
	mux := http.NewServeMux()
	sb.registerRoutes(mux)

	addr := fmt.Sprintf(":%d", sb.config.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Listen for OS shutdown signals in a goroutine so the main goroutine
	// can block on server.ListenAndServe below.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("shutting down — draining connections (10 s max)...")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("forced shutdown: %v", err)
		}
	}()

	log.Printf("listening on http://127.0.0.1%s", addr)

	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}

	log.Println("server stopped")
	return nil
}

// registerRoutes wires every API endpoint to its handler on the given mux.
//
// Go 1.22+ ServeMux supports method + path-parameter patterns:
//
//	"GET /api/collections/{name}/records"  →  only matches GET requests
//	"{name}" captures the path segment into r.PathValue("name")
func (sb *SimpleBase) registerRoutes(mux *http.ServeMux) {
	// ------------------------------------------------------------------ misc
	mux.HandleFunc("GET /api/health", sb.handleHealth)

	// ------------------------------------------------------- collections CRUD
	mux.HandleFunc("GET /api/collections", sb.handleListCollections)
	mux.HandleFunc("POST /api/collections", sb.handleCreateCollection)
	mux.HandleFunc("GET /api/collections/{name}", sb.handleViewCollection)
	mux.HandleFunc("PATCH /api/collections/{name}", sb.handleUpdateCollection)
	mux.HandleFunc("DELETE /api/collections/{name}", sb.handleDeleteCollection)

	// --------------------------------------------------------- records (CRUD)
	mux.HandleFunc("GET /api/collections/{name}/records",
		sb.handleListRecords)
	mux.HandleFunc("POST /api/collections/{name}/records",
		sb.handleCreateRecord)
	mux.HandleFunc("GET /api/collections/{name}/records/{id}",
		sb.handleViewRecord)
	mux.HandleFunc("PATCH /api/collections/{name}/records/{id}",
		sb.handleUpdateRecord)
	mux.HandleFunc("DELETE /api/collections/{name}/records/{id}",
		sb.handleDeleteRecord)

	// ---------------------------------------------------------- auth endpoints
	mux.HandleFunc("GET /api/collections/{name}/auth-methods",
		sb.handleListAuthMethods)
	mux.HandleFunc("POST /api/collections/{name}/auth-with-password",
		sb.handleAuthWithPassword)
	mux.HandleFunc("POST /api/collections/{name}/auth-refresh",
		sb.handleAuthRefresh)
	mux.HandleFunc("POST /api/collections/{name}/request-verification",
		sb.handleRequestVerification)
	mux.HandleFunc("POST /api/collections/{name}/confirm-verification",
		sb.handleConfirmVerification)
	mux.HandleFunc("POST /api/collections/{name}/request-password-reset",
		sb.handleRequestPasswordReset)
	mux.HandleFunc("POST /api/collections/{name}/confirm-password-reset",
		sb.handleConfirmPasswordReset)
	mux.HandleFunc("POST /api/collections/{name}/request-otp",
		sb.handleRequestOTP)
	mux.HandleFunc("POST /api/collections/{name}/auth-with-otp",
		sb.handleAuthWithOTP)
}
