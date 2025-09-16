-- Database Setup Script for Payment System
-- Run this script to set up a fresh database or update an existing one

\echo 'Setting up Payment System Database...'

-- Run the updated schema
\i 003_updated_schema.sql

\echo 'Database setup completed!'

-- Show sample data
\echo 'Sample wallets:'
SELECT
    id,
    user_id,
    balance,
    currency,
    status
FROM wallets
ORDER BY created_at;

\echo 'Sample movements:'
SELECT
    id,
    wallet_id,
    type,
    amount,
    currency,
    reference,
    description
FROM wallet_movements
ORDER BY created_at;

\echo ''
\echo 'Database is ready for use!'
\echo 'Test wallet UUIDs for development:'
\echo '- Wallet 1: 550e8400-e29b-41d4-a716-446655440001 (User: 550e8400-e29b-41d4-a716-446655440010)'
\echo '- Wallet 2: 550e8400-e29b-41d4-a716-446655440002 (User: 550e8400-e29b-41d4-a716-446655440011)'
\echo '- Wallet 3: 550e8400-e29b-41d4-a716-446655440003 (User: 550e8400-e29b-41d4-a716-446655440012)'