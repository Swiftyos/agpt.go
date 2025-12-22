-- Rollback: Initial schema for chatbot API

DROP TRIGGER IF EXISTS update_chat_sessions_updated_at ON chat_sessions;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;

DROP FUNCTION IF EXISTS clean_expired_tokens();
DROP FUNCTION IF EXISTS clean_expired_cache();
DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS cache;
DROP TABLE IF EXISTS chat_messages;
DROP TABLE IF EXISTS chat_sessions;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;

DROP EXTENSION IF EXISTS "uuid-ossp";
