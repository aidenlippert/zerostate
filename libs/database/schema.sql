-- ZeroState Database Schema
-- PostgreSQL 14+
-- CRITICAL: All tables use JSONB for flexibility and audit trails

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================================
-- USERS & AUTHENTICATION
-- ============================================================================

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    did VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE,
    password_hash VARCHAR(255), -- bcrypt hash
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_login_at TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN DEFAULT true,
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_users_did ON users(did);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_active ON users(is_active);

-- JWT refresh tokens for secure authentication
CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    revoked_at TIMESTAMP WITH TIME ZONE,
    ip_address INET,
    user_agent TEXT
);

CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_expires ON refresh_tokens(expires_at);
CREATE INDEX idx_refresh_tokens_token ON refresh_tokens(token_hash);

-- API keys for agent authentication
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(20) NOT NULL, -- First 8 chars for identification
    name VARCHAR(255),
    scopes JSONB DEFAULT '[]'::jsonb, -- ["payments:read", "tasks:execute"]
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    revoked_at TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN DEFAULT true
);

CREATE INDEX idx_api_keys_user ON api_keys(user_id);
CREATE INDEX idx_api_keys_prefix ON api_keys(key_prefix);
CREATE INDEX idx_api_keys_active ON api_keys(is_active);

-- ============================================================================
-- PAYMENT SYSTEM
-- ============================================================================

CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    did VARCHAR(255) UNIQUE NOT NULL,
    balance DECIMAL(20, 8) NOT NULL DEFAULT 0 CHECK (balance >= 0),
    total_deposited DECIMAL(20, 8) NOT NULL DEFAULT 0,
    total_withdrawn DECIMAL(20, 8) NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_accounts_did ON accounts(did);
CREATE INDEX idx_accounts_balance ON accounts(balance);

CREATE TABLE payment_channels (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    payer_did VARCHAR(255) NOT NULL,
    payee_did VARCHAR(255) NOT NULL,
    auction_id VARCHAR(255),
    total_deposit DECIMAL(20, 8) NOT NULL,
    current_balance DECIMAL(20, 8) NOT NULL,
    escrowed_amount DECIMAL(20, 8) NOT NULL DEFAULT 0,
    total_settled DECIMAL(20, 8) NOT NULL DEFAULT 0,
    pending_refund DECIMAL(20, 8) NOT NULL DEFAULT 0,
    state VARCHAR(50) NOT NULL, -- open, escrowed, settling, closed
    task_id VARCHAR(255),
    escrow_released BOOLEAN DEFAULT false,
    sequence_number BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    closed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_channels_payer ON payment_channels(payer_did);
CREATE INDEX idx_channels_payee ON payment_channels(payee_did);
CREATE INDEX idx_channels_auction ON payment_channels(auction_id);
CREATE INDEX idx_channels_task ON payment_channels(task_id);
CREATE INDEX idx_channels_state ON payment_channels(state);

CREATE TABLE channel_transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    channel_id UUID NOT NULL REFERENCES payment_channels(id) ON DELETE CASCADE,
    transaction_type VARCHAR(50) NOT NULL, -- deposit, escrow, release, refund, settle
    amount DECIMAL(20, 8) NOT NULL,
    task_id VARCHAR(255),
    reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_channel_txs_channel ON channel_transactions(channel_id);
CREATE INDEX idx_channel_txs_type ON channel_transactions(transaction_type);
CREATE INDEX idx_channel_txs_created ON channel_transactions(created_at);

-- ============================================================================
-- MARKETPLACE & AGENTS
-- ============================================================================

CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    did VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    capabilities JSONB NOT NULL DEFAULT '[]'::jsonb,
    pricing_model VARCHAR(50),
    status VARCHAR(50) DEFAULT 'online', -- online, busy, offline, maintenance
    max_capacity INT DEFAULT 10,
    current_load INT DEFAULT 0,
    region VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_seen_at TIMESTAMP WITH TIME ZONE,
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_agents_did ON agents(did);
CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_capabilities ON agents USING GIN (capabilities);
CREATE INDEX idx_agents_region ON agents(region);

CREATE TABLE auctions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    auction_type VARCHAR(50) NOT NULL, -- first_price, second_price, reserve
    status VARCHAR(50) NOT NULL, -- open, closed, awarded, canceled, expired
    duration_seconds INT NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    reserve_price DECIMAL(20, 8),
    max_price DECIMAL(20, 8),
    min_reputation DECIMAL(5, 2),
    capabilities JSONB NOT NULL DEFAULT '[]'::jsonb,
    winning_bid_id UUID,
    final_price DECIMAL(20, 8),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_auctions_task ON auctions(task_id);
CREATE INDEX idx_auctions_user ON auctions(user_id);
CREATE INDEX idx_auctions_status ON auctions(status);
CREATE INDEX idx_auctions_expires ON auctions(expires_at);

CREATE TABLE bids (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    auction_id UUID NOT NULL REFERENCES auctions(id) ON DELETE CASCADE,
    agent_did VARCHAR(255) NOT NULL,
    price DECIMAL(20, 8) NOT NULL,
    estimated_time_seconds INT,
    reputation_score DECIMAL(5, 2),
    quality_score DECIMAL(5, 2),
    composite_score DECIMAL(10, 6),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_bids_auction ON bids(auction_id);
CREATE INDEX idx_bids_agent ON bids(agent_did);
CREATE INDEX idx_bids_composite ON bids(composite_score DESC);

-- ============================================================================
-- REPUTATION SYSTEM
-- ============================================================================

CREATE TABLE reputation_scores (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_did VARCHAR(255) UNIQUE NOT NULL,
    overall_score DECIMAL(5, 2) NOT NULL DEFAULT 50.0,
    reliability_score DECIMAL(5, 2) NOT NULL DEFAULT 50.0,
    quality_score DECIMAL(5, 2) NOT NULL DEFAULT 50.0,
    speed_score DECIMAL(5, 2) NOT NULL DEFAULT 50.0,
    total_tasks INT NOT NULL DEFAULT 0,
    successful_tasks INT NOT NULL DEFAULT 0,
    failed_tasks INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_reputation_agent ON reputation_scores(agent_did);
CREATE INDEX idx_reputation_overall ON reputation_scores(overall_score DESC);

CREATE TABLE reputation_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_did VARCHAR(255) NOT NULL,
    event_type VARCHAR(50) NOT NULL, -- success, failure, timeout, quality_feedback
    task_id VARCHAR(255),
    score_delta DECIMAL(5, 2),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_reputation_events_agent ON reputation_events(agent_did);
CREATE INDEX idx_reputation_events_type ON reputation_events(event_type);
CREATE INDEX idx_reputation_events_created ON reputation_events(created_at DESC);

-- ============================================================================
-- TASK EXECUTION
-- ============================================================================

CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id VARCHAR(255) UNIQUE NOT NULL,
    user_did VARCHAR(255) NOT NULL,
    agent_did VARCHAR(255),
    task_type VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL, -- pending, assigned, executing, completed, failed
    input JSONB NOT NULL,
    output JSONB,
    error TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    timeout_at TIMESTAMP WITH TIME ZONE,
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_tasks_task_id ON tasks(task_id);
CREATE INDEX idx_tasks_user ON tasks(user_did);
CREATE INDEX idx_tasks_agent ON tasks(agent_did);
CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_created ON tasks(created_at DESC);

-- ============================================================================
-- AUDIT & LOGGING
-- ============================================================================

CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100),
    resource_id VARCHAR(255),
    ip_address INET,
    user_agent TEXT,
    request_id VARCHAR(255),
    status_code INT,
    error TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_audit_user ON audit_logs(user_id);
CREATE INDEX idx_audit_action ON audit_logs(action);
CREATE INDEX idx_audit_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX idx_audit_created ON audit_logs(created_at DESC);

-- ============================================================================
-- RATE LIMITING
-- ============================================================================

CREATE TABLE rate_limit_buckets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key VARCHAR(255) NOT NULL, -- IP address or user_id
    endpoint VARCHAR(255) NOT NULL,
    tokens_remaining INT NOT NULL,
    window_start TIMESTAMP WITH TIME ZONE NOT NULL,
    window_end TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(key, endpoint, window_start)
);

CREATE INDEX idx_rate_limit_key ON rate_limit_buckets(key, endpoint);
CREATE INDEX idx_rate_limit_window ON rate_limit_buckets(window_end);

-- ============================================================================
-- TRIGGERS FOR UPDATED_AT
-- ============================================================================

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_accounts_updated_at BEFORE UPDATE ON accounts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_payment_channels_updated_at BEFORE UPDATE ON payment_channels
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_agents_updated_at BEFORE UPDATE ON agents
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_auctions_updated_at BEFORE UPDATE ON auctions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_reputation_updated_at BEFORE UPDATE ON reputation_scores
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- VIEWS FOR ANALYTICS
-- ============================================================================

CREATE VIEW agent_performance AS
SELECT
    a.did,
    a.name,
    a.status,
    r.overall_score as reputation,
    r.total_tasks,
    r.successful_tasks,
    r.failed_tasks,
    ROUND(r.successful_tasks::DECIMAL / NULLIF(r.total_tasks, 0) * 100, 2) as success_rate,
    COUNT(DISTINCT b.auction_id) as bids_submitted,
    COUNT(DISTINCT CASE WHEN au.winning_bid_id = b.id THEN au.id END) as auctions_won
FROM agents a
LEFT JOIN reputation_scores r ON a.did = r.agent_did
LEFT JOIN bids b ON a.did = b.agent_did
LEFT JOIN auctions au ON b.auction_id = au.id
GROUP BY a.did, a.name, a.status, r.overall_score, r.total_tasks, r.successful_tasks, r.failed_tasks;

CREATE VIEW payment_statistics AS
SELECT
    DATE_TRUNC('day', created_at) as date,
    COUNT(*) as total_transactions,
    SUM(CASE WHEN transaction_type = 'deposit' THEN amount ELSE 0 END) as total_deposits,
    SUM(CASE WHEN transaction_type = 'settle' THEN amount ELSE 0 END) as total_settlements,
    SUM(CASE WHEN transaction_type = 'refund' THEN amount ELSE 0 END) as total_refunds,
    AVG(CASE WHEN transaction_type = 'settle' THEN amount END) as avg_settlement
FROM channel_transactions
GROUP BY DATE_TRUNC('day', created_at)
ORDER BY date DESC;

-- ============================================================================
-- INITIAL DATA
-- ============================================================================

-- Insert system user for internal operations
INSERT INTO users (did, email, is_active, metadata) VALUES
    ('did:zerostate:system', 'system@zerostate.io', true, '{"type": "system"}'::jsonb);

-- ============================================================================
-- SECURITY: Row-Level Security (Future Enhancement)
-- ============================================================================

-- ALTER TABLE users ENABLE ROW LEVEL SECURITY;
-- ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
-- ALTER TABLE payment_channels ENABLE ROW LEVEL SECURITY;
--
-- CREATE POLICY users_own_data ON users
--     FOR ALL USING (id = current_user_id());
--
-- CREATE POLICY accounts_own_data ON accounts
--     FOR ALL USING (did IN (SELECT did FROM users WHERE id = current_user_id()));
