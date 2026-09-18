package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/data", handleData)

	addr := ":8080"
	log.Printf("simple-api listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// handleData generates a ~5KB JSON payload with random data on every request.
func handleData(w http.ResponseWriter, r *http.Request) {
	type Item struct {
		ID        int       `json:"id"`
		Name      string    `json:"name"`
		Value     float64   `json:"value"`
		Tags      []string  `json:"tags"`
		Timestamp time.Time `json:"timestamp"`
	}

	items := make([]Item, 20)
	for i := range items {
		items[i] = Item{
			ID:        i + 1,
			Name:      randomString(24),
			Value:     rand.Float64() * 10000,
			Tags:      []string{randomString(8), randomString(8), randomString(8)},
			Timestamp: time.Now(),
		}
	}

	payload := struct {
		Count int    `json:"count"`
		Items []Item `json:"items"`
	}{
		Count: len(items),
		Items: items,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, fmt.Sprintf("encoding error: %v", err), http.StatusInternalServerError)
	}
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
