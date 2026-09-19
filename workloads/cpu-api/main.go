package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/compute", handleCompute)

	addr := ":8080"
	log.Printf("cpu-api listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// handleCompute performs a deliberately expensive computation:
// counts all prime numbers up to 50,000 using trial division.
// This reliably pegs the CPU when hit concurrently.
func handleCompute(w http.ResponseWriter, r *http.Request) {
	limit := 500000
	count := countPrimes(limit)

	result := struct {
		Limit  int `json:"limit"`
		Primes int `json:"primes"`
	}{
		Limit:  limit,
		Primes: count,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, fmt.Sprintf("encoding error: %v", err), http.StatusInternalServerError)
	}
}

// countPrimes counts primes up to n using trial division.
// Intentionally naive to maximize CPU work per request.
func countPrimes(n int) int {
	count := 0
	for i := 2; i <= n; i++ {
		if isPrime(i) {
			count++
		}
	}
	return count
}

func isPrime(n int) bool {
	if n < 2 {
		return false
	}
	for i := 2; i*i <= n; i++ {
		if n%i == 0 {
			return false
		}
	}
	return true
}
