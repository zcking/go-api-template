-- Create sequence for users table
CREATE SEQUENCE IF NOT EXISTS seq_users_id START 1;

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY DEFAULT nextval('seq_users_id'),
    email TEXT NOT NULL,
    name TEXT NOT NULL
);
