package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"payment-api/db"
	"payment-api/handler"
)

func healthHandler(w http.ResponseWriter, r *http.Request){
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":"ok",
	})
}

func main(){
	db.Connect()

	mux := http.NewServeMux()
	mux.HandleFunc("/health",healthHandler)
	mux.HandleFunc("/payments", handler.CreatePayment)

	fmt.Println("Server running on port 8080")
	log.Fatal(http.ListenAndServe(":8080",mux))
}