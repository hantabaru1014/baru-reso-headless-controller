CREATE TABLE IF NOT EXISTS registration_tokens (
    token text PRIMARY KEY,
    resonite_id text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    used_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT current_timestamp
);

CREATE INDEX idx_registration_tokens_resonite_id ON registration_tokens(resonite_id);
