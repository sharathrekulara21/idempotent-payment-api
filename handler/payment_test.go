package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"payment-api/db"
)

func setup(t *testing.T) {
	db.Connect()

	// Clean slate before each test
	_, err := db.DB.Exec(context.Background(), "DELETE FROM payments")
	if err != nil {
		t.Fatalf("Failed to clean payments: %v", err)
	}
	_, err = db.DB.Exec(context.Background(), "DELETE FROM idempotency_keys")
	if err != nil {
		t.Fatalf("Failed to clean idempotency_keys: %v", err)
	}
}

func TestCreatePayment_HappyPath(t *testing.T) {
	setup(t)

	body := `{"amount": 100.00, "currency": "INR"}`
	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "test-happy-001")

	rr := httptest.NewRecorder()
	CreatePayment(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected 201, got %d", rr.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)

	if response["id"] == "" {
		t.Error("Expected payment ID in response")
	}
	if response["status"] != "PENDING" {
		t.Errorf("Expected status PENDING, got %v", response["status"])
	}
}

func TestCreatePayment_MissingIdempotencyKey(t *testing.T) {
	setup(t)

	body := `{"amount": 100.00, "currency": "INR"}`
	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	CreatePayment(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
}

func TestCreatePayment_DuplicateKey_ReturnsSameResponse(t *testing.T) {
	setup(t)

	body := `{"amount": 100.00, "currency": "INR"}`

	// First request
	req1 := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBufferString(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", "test-duplicate-001")
	rr1 := httptest.NewRecorder()
	CreatePayment(rr1, req1)

	// Second request — same key
	req2 := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "test-duplicate-001")
	rr2 := httptest.NewRecorder()
	CreatePayment(rr2, req2)

	var resp1, resp2 map[string]interface{}
	json.NewDecoder(rr1.Body).Decode(&resp1)
	json.NewDecoder(rr2.Body).Decode(&resp2)

	if resp1["id"] != resp2["id"] {
		t.Errorf("Expected same payment ID\nFirst: %v\nSecond: %v",
			resp1["id"], resp2["id"])
	}

	if resp1["amount"] != resp2["amount"] {
		t.Errorf("Expected same amount\nFirst: %v\nSecond: %v",
			resp1["amount"], resp2["amount"])
	}

	// Second should have replay header
	if rr2.Header().Get("X-Idempotent-Replay") != "true" {
		t.Error("Expected X-Idempotent-Replay header on duplicate request")
	}

	// Only one payment should exist in DB
	var count int
	db.DB.QueryRow(context.Background(), "SELECT COUNT(*) FROM payments").Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 payment in DB, got %d", count)
	}
}

func TestCreatePayment_ConcurrentRequests_OnlyOnePaymentCreated(t *testing.T) {
	setup(t)

	concurrency := 50
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			body := `{"amount": 100.00, "currency": "INR"}`
			req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "test-concurrent-001")
			rr := httptest.NewRecorder()
			CreatePayment(rr, req)
		}()
	}

	wg.Wait()

	// Assert only one payment was created
	var count int
	db.DB.QueryRow(context.Background(), "SELECT COUNT(*) FROM payments").Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 payment, got %d — race condition not fixed", count)
	}
}