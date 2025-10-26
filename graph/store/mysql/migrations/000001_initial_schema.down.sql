-- Migration: 000001_initial_schema.down.sql
-- Description: Rollback initial schema creation
-- Created: 2025-10-25

-- Drop tables in reverse order of creation
DROP TABLE IF EXISTS workflow_checkpoints;
DROP TABLE IF EXISTS workflow_steps;
