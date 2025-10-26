-- Migration: 000001_initial_schema.up.sql
-- Description: Create initial LangGraph-Go workflow tables
-- Created: 2025-10-25

-- Create workflow_steps table for step-by-step execution history
CREATE TABLE IF NOT EXISTS workflow_steps (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    run_id VARCHAR(255) NOT NULL COMMENT 'Unique identifier for workflow execution',
    step INT NOT NULL COMMENT 'Step number in workflow (0-based)',
    node_id VARCHAR(255) NOT NULL COMMENT 'Node that executed this step',
    state JSON NOT NULL COMMENT 'Serialized workflow state',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT 'When step was created',

    -- Indexes for fast lookup
    INDEX idx_run_id (run_id),
    INDEX idx_run_step (run_id, step),

    -- Ensure no duplicate steps
    UNIQUE KEY unique_run_step (run_id, step)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
COMMENT='Stores step-by-step workflow execution history';

-- Create workflow_checkpoints table for named checkpoints
CREATE TABLE IF NOT EXISTS workflow_checkpoints (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    checkpoint_id VARCHAR(255) NOT NULL UNIQUE COMMENT 'Unique checkpoint identifier',
    state JSON NOT NULL COMMENT 'Serialized state at checkpoint',
    step INT NOT NULL COMMENT 'Step number when checkpoint was created',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT 'When checkpoint was created',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last update time'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
COMMENT='Stores named checkpoints for workflow resumption';
