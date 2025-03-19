package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type APIMapping struct {
	LocalAPI string `json:"local_api"`
}

var (
	apiStore      = make(map[string]string)        // Maps API Key -> Local API URL
	clientSockets = make(map[string]*websocket.Conn) // Maps API Key -> WebSocket Connection
	mutex         = sync.Mutex{}
	upgrader      = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow WebSocket connections from any origin
		},
	}
)

// Enable CORS
func enableCORS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*") // Allow all origins
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
}

// Handle WebSocket Connection (User 1)
func wsHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w, r)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	// Read API Key from WebSocket
	var apiKey string
	err = conn.ReadJSON(&apiKey)
	if err != nil {
		log.Println("Failed to read API Key:", err)
		return
	}

	// Store WebSocket Connection
	mutex.Lock()
	clientSockets[apiKey] = conn
	mutex.Unlock()

	log.Println("User 1 connected for API:", apiKey)

	// Listen for disconnection
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Println("User 1 disconnected:", err)
			mutex.Lock()
			delete(clientSockets, apiKey)
			mutex.Unlock()
			break
		}
	}
}

// Register User 1's Local API
func registerAPI(w http.ResponseWriter, r *http.Request) {
	enableCORS(w, r)

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

	publicURL := fmt.Sprintf("https://api-share-prototype-1-jr7j.onrender.com/api/%s", apiKey)
	json.NewEncoder(w).Encode(map[string]string{
		"public_api": publicURL,
		"ws_url":     fmt.Sprintf("wss://api-share-prototype-1-jr7j.onrender.com/ws/%s", apiKey),
	})
}

// Handle API Call from User 2
func proxyRequest(w http.ResponseWriter, r *http.Request) {
	enableCORS(w, r)

	apiKey := r.URL.Path[len("/api/"):] // Extract API key

	mutex.Lock()
	localAPI, exists := apiStore[apiKey]
	wsConn, wsExists := clientSockets[apiKey]
	mutex.Unlock()

	if !exists {
		http.Error(w, "API not found", http.StatusNotFound)
		return
	}

	// Send request details via WebSocket if connected
	if wsExists {
		reqData := map[string]interface{}{
			"method":  r.Method,
			"url":     localAPI,
			"headers": r.Header,
		}

		// Read request body
		body, _ := io.ReadAll(r.Body)
		reqData["body"] = string(body)

		err := wsConn.WriteJSON(reqData)
		if err != nil {
			log.Println("Failed to send request via WebSocket:", err)
		}

		// Receive response from WebSocket
		var responseData map[string]interface{}
		err = wsConn.ReadJSON(&responseData)
		if err != nil {
			http.Error(w, "Failed to receive response from User 1", http.StatusBadGateway)
			return
		}

		// Send response back to User 2
		w.WriteHeader(int(responseData["status"].(float64)))
		json.NewEncoder(w).Encode(responseData["body"])
		return
	}

	http.Error(w, "User 1 is not online", http.StatusServiceUnavailable)
}

func main() {
	http.HandleFunc("/register", registerAPI) // Register User 1's API
	http.HandleFunc("/api/", proxyRequest)    // Proxy API request
	http.HandleFunc("/ws/", wsHandler)        // WebSocket Connection

	port := "8080"
	fmt.Println("Server running on port:", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
