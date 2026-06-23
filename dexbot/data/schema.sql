/*
Filename: data/schema.sql

Description:
Core database schema for:
✅ trading
✅ risk calculation
✅ training (ML / GA / GP)

Safe for:
- backtesting
- real trading
*/

-- =========================
-- MARKET DATA
-- =========================
CREATE TABLE IF NOT EXISTS market_prices (
    id SERIAL PRIMARY KEY,
    token TEXT,
    price DOUBLE PRECISION,
    ts TIMESTAMP DEFAULT NOW()
);

-- =========================
-- RETURNS (for risk calc)
-- =========================
CREATE TABLE IF NOT EXISTS returns (
    id SERIAL PRIMARY KEY,
    token TEXT,
    return DOUBLE PRECISION,
    ts TIMESTAMP DEFAULT NOW()
);

-- =========================
-- COVARIANCE MATRIX
-- =========================
CREATE TABLE IF NOT EXISTS covariance (
    id SERIAL PRIMARY KEY,
    token_a TEXT,
    token_b TEXT,
    value DOUBLE PRECISION,
    ts TIMESTAMP DEFAULT NOW()
);

-- =========================
-- PORTFOLIO
-- =========================
CREATE TABLE IF NOT EXISTS portfolios (
    id TEXT PRIMARY KEY,
    capital DOUBLE PRECISION,
    value DOUBLE PRECISION,
    created_at TIMESTAMP DEFAULT NOW()
);

-- =========================
-- PORTFOLIO ASSETS
-- =========================
CREATE TABLE IF NOT EXISTS portfolio_assets (
    portfolio_id TEXT,
    token TEXT,
    weight DOUBLE PRECISION,
    quantity DOUBLE PRECISION
);

-- =========================
-- STRATEGY / MODEL
-- =========================
CREATE TABLE IF NOT EXISTS models (
    name TEXT PRIMARY KEY,
    score DOUBLE PRECISION,
    win_rate DOUBLE PRECISION
);

-- =========================
-- MODEL PERFORMANCE LOG
-- =========================
CREATE TABLE IF NOT EXISTS model_logs (
    id SERIAL PRIMARY KEY,
    model TEXT,
    correct BOOLEAN,
    reward DOUBLE PRECISION,
    ts TIMESTAMP DEFAULT NOW()
);
