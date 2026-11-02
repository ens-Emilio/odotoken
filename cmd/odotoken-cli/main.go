package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// HealthStatus mirrors the server's health dashboard response.
type HealthStatus struct {
	Status      string    `json:"status"`
	Uptime      string    `json:"uptime"`
	DBHealthy   bool      `json:"db_healthy"`
	LastCheck   time.Time `json:"last_check"`
	Version     string    `json:"version"`
	ActiveKeys  int       `json:"active_keys"`
	TotalEvents int64     `json:"total_events"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	serverURL := os.Getenv("ODOTOKEN_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	switch os.Args[1] {
	case "health":
		checkHealth(serverURL)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("odotoken-cli — OdoToken management CLI")
	fmt.Println()
	fmt.Println("Usage: odotoken-cli <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  health    Check server health status")
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Println("  ODOTOKEN_URL  Server URL (default: http://localhost:8080)")
}

func checkHealth(serverURL string) {
	resp, err := http.Get(serverURL + "/health")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to server: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Server returned status %d\n", resp.StatusCode)
		os.Exit(1)
	}

	var status HealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding response: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Status: %s\n", status.Status)
	fmt.Printf("Version: %s\n", status.Version)
	fmt.Printf("Uptime: %s\n", status.Uptime)
	fmt.Printf("DB Healthy: %v\n", status.DBHealthy)
	fmt.Printf("Total Events (24h): %d\n", status.TotalEvents)
	fmt.Printf("Last Check: %s\n", status.LastCheck.Format(time.RFC3339))
}