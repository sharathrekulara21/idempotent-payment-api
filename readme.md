# Idempotent Payment API

## What it does

This is a high-performance, production-ready REST API built in Go designed to handle financial transactions safely. Every payment endpoint is strictly protected by an Idempotency-Key HTTP header. If a client sends the exact same payment request multiple times with the same key, the API guarantees that the transaction is processed exactly once, safely returning the cached result for all subsequent duplicate calls without causing double charges.

## The Problem: Race Conditions in Payment Systems

In payment infrastructure, network timeouts and retries are inevitable. If a client sends a payment request, the server processes it successfully, but the network drops before the client receives the 200 OK response, the client will naturally retry.

Without robust idempotency, this retry can lead to serious issues:

1. Double Charging: The user is billed twice for a single purchase.

2. Data Corruption: Duplicate ledger entries are created.

3. Race Conditions: If the user rapidly clicks "Pay" twice, two concurrent requests hit the server simultaneously. Traditional "check-then-insert" logic (checking if a key exists in Go, then inserting it into the database) suffers from a time-of-check to time-of-use (TOCTOU) flaw. Both requests might see that the key doesn't exist yet, pass the check, and both charge the user.

## How it works

- First request flow
  The client sends a POST /payments request with a unique UUID in the Idempotency-Key header.

The server checks the database for the key. Finding nothing, it inserts a new idempotency record marked as PROCESSING.

The server executes the stores the payment record in the PAYMENTS Table.

The API returns the fresh response to the client.

- Duplicate request flow

The client retries the same POST /payments request with the identical Idempotency-Key.

The server attempts to insert the key with ON CONFLICT DO NOTHING — Postgres
silently skips the insert and returns RowsAffected() = 0.

The server recognizes it lost the race, reads the stored response from the
idempotency_keys table, and returns it directly with an X-Idempotent-Replay: true
header — no new payment is created.

- Concurrent request flow

50 simultaneous requests arrive with the same Idempotency-Key.

All 50 attempt the INSERT simultaneously. Postgres's PRIMARY KEY constraint
guarantees exactly one insert succeeds — RowsAffected() = 1 for the winner,
0 for all others.

The winner processes the payment and updates the idempotency record to DONE.

The 49 losers either receive a 409 Conflict with a Retry-After: 1 header
if the winner is still processing, or the cached response if it has already
finished.

Result: exactly 1 payment created regardless of concurrency. Proven by a
50-goroutine stress test in the test suite.

## The Fix: ON CONFLICT + RowsAffected()

The naive fix is to check if a key exists before inserting — but this is a
TOCTOU race condition at the application level.

The correct fix pushes the uniqueness guarantee down to the database layer:

```INSERT INTO idempotency_keys (key, ...) VALUES (...) ON CONFLICT (key) DO NOTHING```

Since key is a PRIMARY KEY, Postgres enforces uniqueness atomically. No two
requests can both win this insert simultaneously — it is physically impossible.

RowsAffected() = 1 means you won. RowsAffected() = 0 means someone else did.
No mutexes, no application-level locks, no distributed coordination needed.

## Running locally

Prerequisites: Go 1.21+, Docker

1. Start Postgres:
   ```
   docker run --name payment-db -e POSTGRES_USER=admin -e POSTGRES_PASSWORD=admin123
   -e POSTGRES_DB=paymentdb -p 5432:5432 -d postgres
   ```

2. Run schema:
   ```docker exec -it payment-db psql -U admin -d paymentdb -f schema.sql```

3. Start the server:
   ```go run main.go```

Server runs on http://localhost:8080

## Running tests

```go test ./handler/ -v```

Tests cover:

- Happy path: single payment creation
- Missing Idempotency-Key: rejected with 400
- Duplicate key: returns cached response with X-Idempotent-Replay header
- Concurrent stress test: 50 goroutines, asserts exactly 1 payment created
