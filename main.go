package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
)

type APIMapping struct {
	LocalAPI string `json:"local_api"`
}

var (
	apiStore = make(map[string]string)
	mutex    = sync.Mutex{}
)

// Enable CORS
func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// Handle OPTIONS request for CORS preflight
func handleOptions(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	w.WriteHeader(http.StatusNoContent)
}

// Register User 1's Local API
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

	publicURL := fmt.Sprintf("http://localhost:8080/api/%s", apiKey)
	json.NewEncoder(w).Encode(map[string]string{"public_api": publicURL})
}

// Handle API Call from User 2
func proxyRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		handleOptions(w, r)
		return
	}

	enableCORS(w)

	apiKey := r.URL.Path[len("/api/"):] // Extract API key

	mutex.Lock()
	localAPI, exists := apiStore[apiKey]
	mutex.Unlock()

	if !exists {
		http.Error(w, "API not found", http.StatusNotFound)
		return
	}

	// Directly call User 1's local API
	resp, err := http.Get(localAPI)
	if err != nil {
		http.Error(w, "Failed to reach User 1's API", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Forward response to User 2
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}

func main() {
	http.HandleFunc("/register", registerAPI) // Register User 1's API
	http.HandleFunc("/api/", proxyRequest)    // User 2 calls API

	fmt.Println("Server running on localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
