CREATE TABLE idempotency_keys (
    key VARCHAR(255) PRIMARY KEY,
    response_body JSONB NOT NULL,
    status_code INT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    amount NUMERIC(12, 2) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);