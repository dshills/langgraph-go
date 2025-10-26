-- LangGraph-Go MySQL Store - Teardown Script
-- This script removes all tables created by the MySQL Store
--
-- ⚠️  WARNING: This will DELETE ALL workflow data! ⚠️
--
-- Usage:
--   mysql -u username -p database_name < teardown.sql
--
-- Or from MySQL shell:
--   SOURCE /path/to/teardown.sql;

-- ============================================================================
-- Safety Check
-- ============================================================================

-- Verify you're in the correct database
SELECT
    'You are about to DROP tables in database:' AS Warning,
    DATABASE() AS Database_Name;

-- Wait for confirmation
-- To proceed, you must comment out this line:
SELECT 'SAFETY CHECK: Comment out this line in teardown.sql to proceed' AS Error;

-- ============================================================================
-- Drop Tables
-- ============================================================================

-- Drop in reverse order of creation to respect dependencies
DROP TABLE IF EXISTS workflow_checkpoints;
DROP TABLE IF EXISTS workflow_steps;

-- ============================================================================
-- Verify Teardown
-- ============================================================================

SELECT 'Tables dropped successfully' AS Status;
SHOW TABLES LIKE 'workflow_%';

SELECT 'Teardown complete!' AS Status;
