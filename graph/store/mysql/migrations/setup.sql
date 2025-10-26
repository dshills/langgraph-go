-- LangGraph-Go MySQL Store - Complete Setup Script
-- This script creates all tables needed for the MySQL Store
--
-- Usage:
--   mysql -u username -p database_name < setup.sql
--
-- Or from MySQL shell:
--   SOURCE /path/to/setup.sql;

-- ============================================================================
-- Database Configuration
-- ============================================================================

-- Ensure we're using UTF-8 MB4 for full Unicode support
SET NAMES utf8mb4;
SET CHARACTER SET utf8mb4;

-- ============================================================================
-- Table: workflow_steps
-- Purpose: Stores step-by-step execution history for workflows
-- ============================================================================

CREATE TABLE IF NOT EXISTS workflow_steps (
    -- Primary key
    id BIGINT AUTO_INCREMENT PRIMARY KEY,

    -- Workflow identification
    run_id VARCHAR(255) NOT NULL COMMENT 'Unique identifier for workflow execution',
    step INT NOT NULL COMMENT 'Step number in workflow (0-based)',
    node_id VARCHAR(255) NOT NULL COMMENT 'Node that executed this step',

    -- State storage
    state JSON NOT NULL COMMENT 'Serialized workflow state at this step',

    -- Metadata
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT 'When step was created',

    -- Indexes for performance
    INDEX idx_run_id (run_id) COMMENT 'Fast lookup by run ID',
    INDEX idx_run_step (run_id, step) COMMENT 'Fast lookup by run ID + step',

    -- Constraints
    UNIQUE KEY unique_run_step (run_id, step) COMMENT 'Prevent duplicate steps'

) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_unicode_ci
  COMMENT='Stores step-by-step workflow execution history';

-- ============================================================================
-- Table: workflow_checkpoints
-- Purpose: Stores named checkpoints for workflow resumption
-- ============================================================================

CREATE TABLE IF NOT EXISTS workflow_checkpoints (
    -- Primary key
    id BIGINT AUTO_INCREMENT PRIMARY KEY,

    -- Checkpoint identification
    checkpoint_id VARCHAR(255) NOT NULL UNIQUE COMMENT 'Unique checkpoint identifier',

    -- State storage
    state JSON NOT NULL COMMENT 'Serialized state at checkpoint',
    step INT NOT NULL COMMENT 'Step number when checkpoint was created',

    -- Metadata
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT 'When checkpoint was created',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last update time'

) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_unicode_ci
  COMMENT='Stores named checkpoints for workflow resumption';

-- ============================================================================
-- Verify Setup
-- ============================================================================

-- Show created tables
SELECT 'Tables created successfully:' AS Status;
SHOW TABLES LIKE 'workflow_%';

-- Show table structures
SELECT 'Table structure: workflow_steps' AS Info;
DESCRIBE workflow_steps;

SELECT 'Table structure: workflow_checkpoints' AS Info;
DESCRIBE workflow_checkpoints;

-- Display summary
SELECT
    'Setup complete!' AS Status,
    @@version AS MySQL_Version,
    DATABASE() AS Database_Name;
