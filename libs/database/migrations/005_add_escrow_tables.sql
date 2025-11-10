-- Migration 005: Add escrow and dispute resolution tables
--
-- Creates tables to support secure payment escrow, dispute resolution,
-- and evidence tracking for economic transactions.

-- Escrows table: tracks secure payment escrow transactions
CREATE TABLE IF NOT EXISTS escrows (
	id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
	task_id TEXT NOT NULL,
	payer_id TEXT NOT NULL,
	payee_id TEXT NOT NULL,
	amount DECIMAL(10, 4) NOT NULL,
	status TEXT DEFAULT 'created',
	funded_at TIMESTAMPTZ,
	released_at TIMESTAMPTZ,
	refunded_at TIMESTAMPTZ,
	dispute_id UUID,
	expires_at TIMESTAMPTZ NOT NULL,
	auto_release_at TIMESTAMPTZ,
	conditions TEXT,
	created_at TIMESTAMPTZ DEFAULT NOW(),
	updated_at TIMESTAMPTZ DEFAULT NOW(),
	error TEXT
);

-- Index for task_id lookup
CREATE INDEX IF NOT EXISTS idx_escrows_task_id ON escrows(task_id);

-- Index for payer_id lookup
CREATE INDEX IF NOT EXISTS idx_escrows_payer_id ON escrows(payer_id);

-- Index for payee_id lookup
CREATE INDEX IF NOT EXISTS idx_escrows_payee_id ON escrows(payee_id);

-- Index for status filtering
CREATE INDEX IF NOT EXISTS idx_escrows_status ON escrows(status);

-- Index for auto-release processing
CREATE INDEX IF NOT EXISTS idx_escrows_auto_release ON escrows(auto_release_at)
WHERE status = 'funded' AND auto_release_at IS NOT NULL;

-- Disputes table: tracks disputes on escrow transactions
CREATE TABLE IF NOT EXISTS disputes (
	id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
	escrow_id UUID NOT NULL REFERENCES escrows(id) ON DELETE CASCADE,
	initiator_id TEXT NOT NULL,
	reason TEXT NOT NULL,
	status TEXT DEFAULT 'open',
	reviewer_id TEXT,
	resolution TEXT,
	resolved_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ DEFAULT NOW(),
	updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for escrow_id lookup
CREATE INDEX IF NOT EXISTS idx_disputes_escrow_id ON disputes(escrow_id);

-- Index for initiator_id lookup
CREATE INDEX IF NOT EXISTS idx_disputes_initiator_id ON disputes(initiator_id);

-- Index for status filtering
CREATE INDEX IF NOT EXISTS idx_disputes_status ON disputes(status);

-- Index for reviewer assignment
CREATE INDEX IF NOT EXISTS idx_disputes_reviewer_id ON disputes(reviewer_id);

-- Dispute evidence table: stores evidence submitted for disputes
CREATE TABLE IF NOT EXISTS dispute_evidence (
	id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
	dispute_id UUID NOT NULL REFERENCES disputes(id) ON DELETE CASCADE,
	submitter_id TEXT NOT NULL,
	evidence_type TEXT NOT NULL,
	content TEXT NOT NULL,
	file_url TEXT,
	created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for dispute_id lookup
CREATE INDEX IF NOT EXISTS idx_dispute_evidence_dispute_id ON dispute_evidence(dispute_id);

-- Index for submitter_id lookup
CREATE INDEX IF NOT EXISTS idx_dispute_evidence_submitter_id ON dispute_evidence(submitter_id);

-- Comments for documentation
COMMENT ON TABLE escrows IS 'Secure payment escrow transactions with state machine';
COMMENT ON TABLE disputes IS 'Dispute resolution for escrow transactions';
COMMENT ON TABLE dispute_evidence IS 'Evidence submitted for dispute resolution';

COMMENT ON COLUMN escrows.status IS 'Status: created, funded, released, refunded, disputed, cancelled';
COMMENT ON COLUMN escrows.auto_release_at IS 'Timestamp for automatic release (if conditions met)';
COMMENT ON COLUMN escrows.conditions IS 'JSON conditions for automatic release';

COMMENT ON COLUMN disputes.status IS 'Status: open, reviewing, resolved, closed';
COMMENT ON COLUMN disputes.resolution IS 'Final resolution decision and reasoning';

COMMENT ON COLUMN dispute_evidence.evidence_type IS 'Type: text, file, screenshot, log';
