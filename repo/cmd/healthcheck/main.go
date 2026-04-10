package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

// Standalone healthcheck binary for Docker HEALTHCHECK directive.
// Does not depend on curl being installed in the runtime image.
func main() {
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/health/live", port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Health check returned status %d\n", resp.StatusCode)
		os.Exit(1)
	}

	os.Exit(0)
}
