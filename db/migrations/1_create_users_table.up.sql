CREATE TABLE IF NOT EXISTS users (
	id text PRIMARY KEY,
	password text NOT NULL,
  resonite_id text,
  icon_url text,
  created_at timestamp DEFAULT current_timestamp,
  updated_at timestamp DEFAULT current_timestamp
);
