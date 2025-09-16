-- Updated Complete Schema for Payment System
-- This is the current state after removing saga orchestration and adding wallet movements

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create event_stream table for event sourcing
CREATE TABLE IF NOT EXISTS event_stream (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    version VARCHAR(50) NOT NULL DEFAULT '1.0',
    data JSONB NOT NULL,
    metadata JSONB DEFAULT '{}',
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    correlation_id UUID,
    stream_version INTEGER NOT NULL,
    UNIQUE(aggregate_id, stream_version)
);

-- Create indexes for event_stream
CREATE INDEX IF NOT EXISTS idx_event_stream_aggregate_id ON event_stream(aggregate_id);
CREATE INDEX IF NOT EXISTS idx_event_stream_event_type ON event_stream(event_type);
CREATE INDEX IF NOT EXISTS idx_event_stream_timestamp ON event_stream(timestamp);
CREATE INDEX IF NOT EXISTS idx_event_stream_correlation_id ON event_stream(correlation_id);

-- Create snapshots table for event sourcing optimization
CREATE TABLE IF NOT EXISTS snapshots (
    aggregate_id UUID PRIMARY KEY,
    aggregate_type VARCHAR(255) NOT NULL,
    version INTEGER NOT NULL,
    snapshot_data JSONB NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create payments table
CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL,
    amount BIGINT NOT NULL CHECK (amount > 0),
    currency VARCHAR(3) NOT NULL,
    payment_method_type VARCHAR(50) NOT NULL,
    payment_method_wallet_id UUID,
    description TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER NOT NULL DEFAULT 1
);

-- Create indexes for payments
CREATE INDEX IF NOT EXISTS idx_payments_user_id ON payments(user_id);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(status);
CREATE INDEX IF NOT EXISTS idx_payments_created_at ON payments(created_at);
CREATE INDEX IF NOT EXISTS idx_payments_deleted_at ON payments(deleted_at);

-- Create wallets table
CREATE TABLE IF NOT EXISTS wallets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL UNIQUE,
    balance BIGINT NOT NULL DEFAULT 0 CHECK (balance >= 0),
    currency VARCHAR(3) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER NOT NULL DEFAULT 1
);

-- Create indexes for wallets
CREATE INDEX IF NOT EXISTS idx_wallets_user_id ON wallets(user_id);
CREATE INDEX IF NOT EXISTS idx_wallets_status ON wallets(status);
CREATE INDEX IF NOT EXISTS idx_wallets_deleted_at ON wallets(deleted_at);

-- Create wallet_transactions table
CREATE TABLE IF NOT EXISTS wallet_transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    wallet_id UUID NOT NULL REFERENCES wallets(id),
    type VARCHAR(50) NOT NULL,
    amount BIGINT NOT NULL CHECK (amount > 0),
    currency VARCHAR(3) NOT NULL,
    balance_before BIGINT NOT NULL CHECK (balance_before >= 0),
    balance_after BIGINT NOT NULL CHECK (balance_after >= 0),
    reference TEXT NOT NULL,
    payment_id UUID,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create indexes for wallet_transactions
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_wallet_id ON wallet_transactions(wallet_id);
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_payment_id ON wallet_transactions(payment_id);
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_type ON wallet_transactions(type);
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_created_at ON wallet_transactions(created_at);
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_deleted_at ON wallet_transactions(deleted_at);

-- Create wallet_movements table (NEW - as per documentation)
CREATE TABLE IF NOT EXISTS wallet_movements (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    wallet_id UUID NOT NULL REFERENCES wallets(id),
    type VARCHAR(50) NOT NULL CHECK (type IN ('income', 'expense')),
    amount BIGINT NOT NULL CHECK (amount > 0),
    currency VARCHAR(3) NOT NULL,
    transaction_id UUID REFERENCES wallet_transactions(id),
    description TEXT,
    reference TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create indexes for wallet_movements
CREATE INDEX IF NOT EXISTS idx_wallet_movements_wallet_id ON wallet_movements(wallet_id);
CREATE INDEX IF NOT EXISTS idx_wallet_movements_type ON wallet_movements(type);
CREATE INDEX IF NOT EXISTS idx_wallet_movements_transaction_id ON wallet_movements(transaction_id);
CREATE INDEX IF NOT EXISTS idx_wallet_movements_created_at ON wallet_movements(created_at);
CREATE INDEX IF NOT EXISTS idx_wallet_movements_deleted_at ON wallet_movements(deleted_at);

-- Create trigger function to update updated_at timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers for updated_at
CREATE TRIGGER update_payments_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_wallets_updated_at
    BEFORE UPDATE ON wallets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_wallet_transactions_updated_at
    BEFORE UPDATE ON wallet_transactions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_wallet_movements_updated_at
    BEFORE UPDATE ON wallet_movements
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Add constraints
DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'check_payment_method_wallet'
        AND table_name = 'payments'
    ) THEN
        ALTER TABLE payments ADD CONSTRAINT check_payment_method_wallet
            CHECK (
                (payment_method_type = 'wallet' AND payment_method_wallet_id IS NOT NULL) OR
                (payment_method_type != 'wallet')
            );
    END IF;
END $$;

-- Add comments for documentation
COMMENT ON TABLE event_stream IS 'Event sourcing event store';
COMMENT ON TABLE snapshots IS 'Event sourcing snapshots for performance optimization';
COMMENT ON TABLE payments IS 'Payment aggregates';
COMMENT ON TABLE wallets IS 'User wallet aggregates';
COMMENT ON TABLE wallet_transactions IS 'Wallet transaction history (internal transactions)';
COMMENT ON TABLE wallet_movements IS 'Wallet movements as business events (income/expense)';

COMMENT ON COLUMN payments.amount IS 'Amount in cents (smallest currency unit)';
COMMENT ON COLUMN wallets.balance IS 'Balance in cents (smallest currency unit)';
COMMENT ON COLUMN wallet_transactions.amount IS 'Transaction amount in cents';
COMMENT ON COLUMN wallet_transactions.balance_before IS 'Wallet balance before transaction in cents';
COMMENT ON COLUMN wallet_transactions.balance_after IS 'Wallet balance after transaction in cents';
COMMENT ON COLUMN wallet_movements.type IS 'Movement type: income or expense';
COMMENT ON COLUMN wallet_movements.amount IS 'Movement amount in cents (smallest currency unit)';
COMMENT ON COLUMN wallet_movements.transaction_id IS 'Optional reference to the underlying transaction';

-- Insert sample data for testing (only if tables are empty)
INSERT INTO wallets (id, user_id, balance, currency, status)
SELECT '550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440010', 100000, 'USD', 'active'
WHERE NOT EXISTS (SELECT 1 FROM wallets WHERE id = '550e8400-e29b-41d4-a716-446655440001');

INSERT INTO wallets (id, user_id, balance, currency, status)
SELECT '550e8400-e29b-41d4-a716-446655440002', '550e8400-e29b-41d4-a716-446655440011', 50000, 'USD', 'active'
WHERE NOT EXISTS (SELECT 1 FROM wallets WHERE id = '550e8400-e29b-41d4-a716-446655440002');

INSERT INTO wallets (id, user_id, balance, currency, status)
SELECT '550e8400-e29b-41d4-a716-446655440003', '550e8400-e29b-41d4-a716-446655440012', 75000, 'EUR', 'active'
WHERE NOT EXISTS (SELECT 1 FROM wallets WHERE id = '550e8400-e29b-41d4-a716-446655440003');

-- Insert sample movements for testing
INSERT INTO wallet_movements (wallet_id, type, amount, currency, reference, description)
SELECT '550e8400-e29b-41d4-a716-446655440001', 'income', 100000, 'USD', 'Initial balance', 'Starting wallet balance'
WHERE NOT EXISTS (
    SELECT 1 FROM wallet_movements
    WHERE wallet_id = '550e8400-e29b-41d4-a716-446655440001'
    AND reference = 'Initial balance'
);

INSERT INTO wallet_movements (wallet_id, type, amount, currency, reference, description)
SELECT '550e8400-e29b-41d4-a716-446655440002', 'income', 50000, 'USD', 'Initial balance', 'Starting wallet balance'
WHERE NOT EXISTS (
    SELECT 1 FROM wallet_movements
    WHERE wallet_id = '550e8400-e29b-41d4-a716-446655440002'
    AND reference = 'Initial balance'
);

INSERT INTO wallet_movements (wallet_id, type, amount, currency, reference, description)
SELECT '550e8400-e29b-41d4-a716-446655440003', 'income', 75000, 'EUR', 'Initial balance', 'Starting wallet balance'
WHERE NOT EXISTS (
    SELECT 1 FROM wallet_movements
    WHERE wallet_id = '550e8400-e29b-41d4-a716-446655440003'
    AND reference = 'Initial balance'
);