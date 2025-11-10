-- Migration 004: Add delegation and subtasks tables for meta-orchestrator
--
-- Creates tables to support task decomposition, multi-agent coordination,
-- and progress tracking across complex delegated tasks.

-- Delegations table: tracks complex tasks delegated to the meta-orchestrator
CREATE TABLE IF NOT EXISTS delegations (
	id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
	task_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	query TEXT NOT NULL,
	capabilities JSONB DEFAULT '[]'::jsonb,
	budget DECIMAL(10, 4) NOT NULL,
	priority TEXT DEFAULT 'normal',
	status TEXT DEFAULT 'pending',
	agents_count INT DEFAULT 0,
	estimated_completion TIMESTAMPTZ NOT NULL,
	actual_completion TIMESTAMPTZ,
	created_at TIMESTAMPTZ DEFAULT NOW(),
	updated_at TIMESTAMPTZ DEFAULT NOW(),
	error TEXT
);

-- Index for task_id lookup
CREATE INDEX IF NOT EXISTS idx_delegations_task_id ON delegations(task_id);

-- Index for user_id lookup
CREATE INDEX IF NOT EXISTS idx_delegations_user_id ON delegations(user_id);

-- Index for status filtering
CREATE INDEX IF NOT EXISTS idx_delegations_status ON delegations(status);

-- Subtasks table: individual tasks within a delegation
CREATE TABLE IF NOT EXISTS subtasks (
	id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
	delegation_id UUID NOT NULL REFERENCES delegations(id) ON DELETE CASCADE,
	task_id TEXT NOT NULL,
	description TEXT NOT NULL,
	agent_id TEXT,
	status TEXT DEFAULT 'pending',
	budget_share DECIMAL(10, 4) NOT NULL,
	started_at TIMESTAMPTZ,
	completed_at TIMESTAMPTZ,
	result TEXT,
	error TEXT,
	created_at TIMESTAMPTZ DEFAULT NOW(),
	updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for delegation_id lookup
CREATE INDEX IF NOT EXISTS idx_subtasks_delegation_id ON subtasks(delegation_id);

-- Index for agent_id lookup
CREATE INDEX IF NOT EXISTS idx_subtasks_agent_id ON subtasks(agent_id);

-- Index for status filtering
CREATE INDEX IF NOT EXISTS idx_subtasks_status ON subtasks(status);

-- Comments for documentation
COMMENT ON TABLE delegations IS 'Tracks complex tasks delegated to the meta-orchestrator for multi-agent coordination';
COMMENT ON TABLE subtasks IS 'Individual subtasks decomposed from delegated tasks';

COMMENT ON COLUMN delegations.status IS 'Status: pending, planning, in_progress, completed, failed, cancelled';
COMMENT ON COLUMN delegations.priority IS 'Priority: low, normal, high';
COMMENT ON COLUMN delegations.capabilities IS 'JSON array of required agent capabilities';

COMMENT ON COLUMN subtasks.status IS 'Status: pending, assigned, in_progress, completed, failed';
COMMENT ON COLUMN subtasks.budget_share IS 'Portion of total budget allocated to this subtask';
