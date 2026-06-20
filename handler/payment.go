package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"payment-api/db"
	"payment-api/model"
)

func CreatePayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Step 1: Reject if Idempotency-Key is missing
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		http.Error(w, "Idempotency-Key header is required", http.StatusBadRequest)
		return
	}

	// Step 2: Parse and validate request body
	var p model.Payment
	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if p.Amount <= 0 || p.Currency == "" {
		http.Error(w, "Amount and currency are required", http.StatusBadRequest)
		return
	}

	// Step 3: Try to claim the key — track whether WE inserted it
	result, err := db.DB.Exec(
		context.Background(),
		`INSERT INTO idempotency_keys (key, response_body, status_code, status)
		 VALUES ($1, $2, $3, 'PROCESSING')
		 ON CONFLICT (key) DO NOTHING`,
		idempotencyKey, []byte("{}"), 0,
	)

	if err != nil {
		http.Error(w, "Failed to process request", http.StatusInternalServerError)
		return
	}

	weWon := result.RowsAffected() == 1

	if !weWon {
		// Someone else claimed this key — read what they stored
		var storedBody []byte
		var storedStatusCode int
		var storedStatus string

		err = db.DB.QueryRow(
			context.Background(),
			"SELECT response_body, status_code, status FROM idempotency_keys WHERE key = $1",
			idempotencyKey,
		).Scan(&storedBody, &storedStatusCode, &storedStatus)

		if err != nil {
			http.Error(w, "Failed to process request", http.StatusInternalServerError)
			return
		}

		if storedStatus == "DONE" {
			// Already fully processed — return stored response
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Idempotent-Replay", "true")
			w.WriteHeader(storedStatusCode)
			w.Write(storedBody)
			return
		}

		// Still processing — tell client to retry
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Payment is being processed, please retry",
		})
		return
	}

	// Step 4: We won — create the payment
	p.Status = "PENDING"
	err = db.DB.QueryRow(
		context.Background(),
		"INSERT INTO payments (amount, currency, status) VALUES ($1, $2, $3) RETURNING id",
		p.Amount, p.Currency, p.Status,
	).Scan(&p.ID)

	if err != nil {
		log.Printf("Payment insert failed: %v", err)
		http.Error(w, "Failed to create payment", http.StatusInternalServerError)
		return
	}

	// Step 5: Update idempotency key with real response
	responseBody, _ := json.Marshal(p)
	statusCode := http.StatusCreated

	_, err = db.DB.Exec(
		context.Background(),
		`UPDATE idempotency_keys 
		 SET response_body = $1, status_code = $2, status = 'DONE' 
		 WHERE key = $3`,
		responseBody, statusCode, idempotencyKey,
	)

	if err != nil {
		http.Error(w, "Failed to store idempotency key", http.StatusInternalServerError)
		return
	}

	// Step 6: Return response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(responseBody)
}