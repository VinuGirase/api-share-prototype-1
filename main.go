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

// Enable CORS for all responses
func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
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
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	apiKey := fmt.Sprintf("%d", len(apiStore)+1)

	mutex.Lock()
	apiStore[apiKey] = apiData.LocalAPI
	mutex.Unlock()

	baseURL := "https://api-share-prototype-1-jr7j.onrender.com"
	

	publicURL := fmt.Sprintf("%s/api/%s", baseURL, apiKey)
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
		log.Println("ERROR: API key not found:", apiKey)
		return
	}

	log.Println("Proxying request to:", localAPI)

	// Create a request with the same method & body
	req, err := http.NewRequest(r.Method, localAPI, r.Body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		log.Println("ERROR: Failed to create request:", err)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to reach the registered API", http.StatusBadGateway)
		log.Println("ERROR: Failed to reach the registered API:", err)
		return
	}
	defer resp.Body.Close()

	log.Println("Successfully reached API. Status:", resp.StatusCode)

	// Forward response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Forward response status & body
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func main() {
	http.HandleFunc("/register", registerAPI) // Register User 1's API
	http.HandleFunc("/api/", proxyRequest)    // Proxy API request

	port := "8080"
	fmt.Println("Server running on port:", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
