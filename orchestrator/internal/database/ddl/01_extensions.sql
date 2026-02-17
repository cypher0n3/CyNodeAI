-- Idempotent DDL bootstrap: PostgreSQL extensions required by postgres_schema.md.
-- Run after GORM AutoMigrate. Safe to run repeatedly.

CREATE EXTENSION IF NOT EXISTS "vector";
