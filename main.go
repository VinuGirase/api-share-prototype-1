package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

type APIMapping struct {
	LocalAPI string `json:"local_api"`
}

var (
	apiStore = make(map[string]string)
	mutex    = sync.Mutex{}
)

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func handleOptions(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	w.WriteHeader(http.StatusNoContent)
}

func registerAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		handleOptions(w, r)
		return
	}

	enableCORS(w)

	var apiData APIMapping
	err := json.NewDecoder(r.Body).Decode(&apiData)
	if err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	apiKey := fmt.Sprintf("%d", len(apiStore)+1)
	mutex.Lock()
	apiStore[apiKey] = apiData.LocalAPI
	mutex.Unlock()

	publicURL := fmt.Sprintf("/api/%s", apiKey)
	json.NewEncoder(w).Encode(map[string]string{"public_api": publicURL})
}

func proxyRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		handleOptions(w, r)
		return
	}

	enableCORS(w)

	apiKey := r.URL.Path[len("/api/"):]
	mutex.Lock()
	localAPI, exists := apiStore[apiKey]
	mutex.Unlock()

	if !exists {
		http.Error(w, "API not found", http.StatusNotFound)
		return
	}

	resp, err := http.Get(localAPI)
	if err != nil {
		http.Error(w, "Failed to reach User 1's API", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}

func main() {
	port := os.Getenv("PORT") // Get assigned PORT from Render
	if port == "" {
		port = "8080" // Default to 8080 for local testing
	}

	address := fmt.Sprintf("0.0.0.0:%s", port) // Bind to all network interfaces

	http.HandleFunc("/register", registerAPI)
	http.HandleFunc("/api/", proxyRequest)

	fmt.Println("Server running on", address)
	log.Fatal(http.ListenAndServe(address, nil))
}
