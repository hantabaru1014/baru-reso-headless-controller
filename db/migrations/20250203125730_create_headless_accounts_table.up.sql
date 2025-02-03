CREATE TABLE IF NOT EXISTS headless_accounts (
  resonite_id text PRIMARY KEY,
  credential text NOT NULL,
  password text NOT NULL,
  last_display_name text,
  last_icon_url text,
  created_at timestamp with time zone DEFAULT current_timestamp,
  updated_at timestamp with time zone DEFAULT current_timestamp
);
