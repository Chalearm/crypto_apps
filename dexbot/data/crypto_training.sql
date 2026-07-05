/* =====================================================================
   CRYPTO MARKET PREDICTION DATABASE SCHEMA
   Version: 1.0

   Supports:
   - BTC
   - ETH
   - BNB
   - ADA
   - SOL
   - UNI
   - XRP
   - DOGE
   - Any future crypto asset

   Recommended:
   PostgreSQL + TimescaleDB

   Architecture:

   Assets
     ├─ Exchanges
     ├─ OHLCV
     ├─ Trades
     ├─ Order Book
     ├─ Futures Data
     ├─ Sentiment
     ├─ On-Chain Data
     ├─ Features
     ├─ Labels
     ├─ Predictions
     └─ Backtests
===================================================================== */


/* =====================================================================
   SECTION 1 : ASSETS
===================================================================== */

CREATE TABLE assets (
    asset_id SERIAL PRIMARY KEY,

    symbol VARCHAR(30) UNIQUE NOT NULL,

    base_asset VARCHAR(20) NOT NULL,

    quote_asset VARCHAR(20) NOT NULL,

    asset_type VARCHAR(20) NOT NULL,

    is_active BOOLEAN DEFAULT TRUE,

    created_at TIMESTAMPTZ DEFAULT NOW()
);

COMMENT ON TABLE assets IS
'Crypto trading instruments (BTCUSDT, ETHUSDT, ADAUSDT, etc.)';


/* =====================================================================
   SECTION 2 : EXCHANGES
===================================================================== */

CREATE TABLE exchanges (
    exchange_id SERIAL PRIMARY KEY,

    name VARCHAR(50) UNIQUE NOT NULL,

    is_active BOOLEAN DEFAULT TRUE,

    created_at TIMESTAMPTZ DEFAULT NOW()
);

COMMENT ON TABLE exchanges IS
'Binance, Bybit, OKX, Coinbase, Kraken, etc.';


/* =====================================================================
   SECTION 3 : OHLCV 1 MINUTE
===================================================================== */

CREATE TABLE ohlcv_1m (

    ts TIMESTAMPTZ NOT NULL,

    asset_id INTEGER NOT NULL
        REFERENCES assets(asset_id),

    exchange_id INTEGER NOT NULL
        REFERENCES exchanges(exchange_id),

    open NUMERIC(26,12) NOT NULL,
    high NUMERIC(26,12) NOT NULL,
    low NUMERIC(26,12) NOT NULL,
    close NUMERIC(26,12) NOT NULL,

    volume NUMERIC(28,12),

    quote_volume NUMERIC(28,12),

    trade_count INTEGER,

    created_at TIMESTAMPTZ DEFAULT NOW(),

    PRIMARY KEY (
        ts,
        asset_id,
        exchange_id
    )
);

CREATE INDEX idx_ohlcv_asset_ts
ON ohlcv_1m(asset_id, ts DESC);


/* =====================================================================
   SECTION 4 : TRADES
===================================================================== */

CREATE TABLE trades (

    trade_id BIGINT NOT NULL,

    ts TIMESTAMPTZ NOT NULL,

    asset_id INTEGER NOT NULL
        REFERENCES assets(asset_id),

    exchange_id INTEGER NOT NULL
        REFERENCES exchanges(exchange_id),

    price NUMERIC(26,12) NOT NULL,

    quantity NUMERIC(28,12) NOT NULL,

    side SMALLINT NOT NULL,

    created_at TIMESTAMPTZ DEFAULT NOW(),

    PRIMARY KEY (
        trade_id,
        exchange_id
    )
);

COMMENT ON COLUMN trades.side IS
'1 = BUY, -1 = SELL';

CREATE INDEX idx_trades_asset_ts
ON trades(asset_id, ts DESC);


/* =====================================================================
   SECTION 5 : ORDERBOOK SNAPSHOT
===================================================================== */

CREATE TABLE orderbook_snapshot (

    ts TIMESTAMPTZ NOT NULL,

    asset_id INTEGER NOT NULL
        REFERENCES assets(asset_id),

    exchange_id INTEGER NOT NULL
        REFERENCES exchanges(exchange_id),

    best_bid NUMERIC(26,12),

    best_ask NUMERIC(26,12),

    bid_volume NUMERIC(28,12),

    ask_volume NUMERIC(28,12),

    spread NUMERIC(26,12),

    mid_price DOUBLE PRECISION,

    imbalance DOUBLE PRECISION,

    created_at TIMESTAMPTZ DEFAULT NOW(),

    PRIMARY KEY (
        ts,
        asset_id,
        exchange_id
    )
);

CREATE INDEX idx_orderbook_asset_ts
ON orderbook_snapshot(asset_id, ts DESC);


/* =====================================================================
   SECTION 6 : FUTURES / DERIVATIVES
===================================================================== */

CREATE TABLE futures_metrics (

    ts TIMESTAMPTZ NOT NULL,

    asset_id INTEGER NOT NULL
        REFERENCES assets(asset_id),

    exchange_id INTEGER NOT NULL
        REFERENCES exchanges(exchange_id),

    funding_rate DOUBLE PRECISION,

    open_interest DOUBLE PRECISION,

    oi_change DOUBLE PRECISION,

    long_short_ratio DOUBLE PRECISION,

    mark_price DOUBLE PRECISION,

    basis DOUBLE PRECISION,

    liquidation_long DOUBLE PRECISION,

    liquidation_short DOUBLE PRECISION,

    created_at TIMESTAMPTZ DEFAULT NOW(),

    PRIMARY KEY (
        ts,
        asset_id,
        exchange_id
    )
);

CREATE INDEX idx_futures_asset_ts
ON futures_metrics(asset_id, ts DESC);


/* =====================================================================
   SECTION 7 : SENTIMENT FEATURES
===================================================================== */

CREATE TABLE sentiment_features (

    ts TIMESTAMPTZ NOT NULL,

    asset_id INTEGER NOT NULL
        REFERENCES assets(asset_id),

    fear_greed_index DOUBLE PRECISION,

    news_sentiment DOUBLE PRECISION,

    twitter_sentiment DOUBLE PRECISION,

    reddit_sentiment DOUBLE PRECISION,

    social_volume DOUBLE PRECISION,

    created_at TIMESTAMPTZ DEFAULT NOW(),

    PRIMARY KEY (
        ts,
        asset_id
    )
);


/* =====================================================================
   SECTION 8 : ON-CHAIN FEATURES
===================================================================== */

CREATE TABLE onchain_metrics (

    ts TIMESTAMPTZ NOT NULL,

    asset_id INTEGER NOT NULL
        REFERENCES assets(asset_id),

    active_addresses DOUBLE PRECISION,

    transaction_count DOUBLE PRECISION,

    exchange_inflow DOUBLE PRECISION,

    exchange_outflow DOUBLE PRECISION,

    whale_transactions DOUBLE PRECISION,

    network_fees DOUBLE PRECISION,

    hash_rate DOUBLE PRECISION,

    created_at TIMESTAMPTZ DEFAULT NOW(),

    PRIMARY KEY (
        ts,
        asset_id
    )
);


/* =====================================================================
   SECTION 9 : FEATURE STORE
===================================================================== */

CREATE TABLE features_10m (

    ts TIMESTAMPTZ NOT NULL,

    asset_id INTEGER NOT NULL
        REFERENCES assets(asset_id),

    close_price DOUBLE PRECISION,

    return_1m DOUBLE PRECISION,
    return_5m DOUBLE PRECISION,
    return_10m DOUBLE PRECISION,

    return_30m DOUBLE PRECISION,
    return_60m DOUBLE PRECISION,

    volume_10m DOUBLE PRECISION,
    volume_30m DOUBLE PRECISION,

    buy_volume_10m DOUBLE PRECISION,
    sell_volume_10m DOUBLE PRECISION,

    volume_delta DOUBLE PRECISION,

    volatility_10m DOUBLE PRECISION,
    volatility_30m DOUBLE PRECISION,
    volatility_60m DOUBLE PRECISION,

    spread_mean DOUBLE PRECISION,

    spread_std DOUBLE PRECISION,

    orderbook_imbalance DOUBLE PRECISION,

    vwap_10m DOUBLE PRECISION,

    rsi_14 DOUBLE PRECISION,

    macd DOUBLE PRECISION,

    macd_signal DOUBLE PRECISION,

    macd_histogram DOUBLE PRECISION,

    atr DOUBLE PRECISION,

    funding_rate DOUBLE PRECISION,

    open_interest DOUBLE PRECISION,

    oi_change_10m DOUBLE PRECISION,

    long_short_ratio DOUBLE PRECISION,

    fear_greed_index DOUBLE PRECISION,

    news_sentiment DOUBLE PRECISION,

    social_volume DOUBLE PRECISION,

    active_addresses DOUBLE PRECISION,

    exchange_inflow DOUBLE PRECISION,

    exchange_outflow DOUBLE PRECISION,

    created_at TIMESTAMPTZ DEFAULT NOW(),

    PRIMARY KEY (
        ts,
        asset_id
    )
);

CREATE INDEX idx_feature_asset_ts
ON features_10m(asset_id, ts DESC);


/* =====================================================================
   SECTION 10 : LABELS
===================================================================== */

CREATE TABLE labels (

    ts TIMESTAMPTZ NOT NULL,

    asset_id INTEGER NOT NULL
        REFERENCES assets(asset_id),

    current_price DOUBLE PRECISION,

    future_price_10m DOUBLE PRECISION,
    future_price_30m DOUBLE PRECISION,
    future_price_60m DOUBLE PRECISION,

    future_return_10m DOUBLE PRECISION,
    future_return_30m DOUBLE PRECISION,
    future_return_60m DOUBLE PRECISION,

    future_spread_10m DOUBLE PRECISION,

    direction_10m SMALLINT,
    direction_30m SMALLINT,
    direction_60m SMALLINT,

    created_at TIMESTAMPTZ DEFAULT NOW(),

    PRIMARY KEY (
        ts,
        asset_id
    )
);

COMMENT ON COLUMN labels.direction_10m IS
'1=UP, 0=FLAT, -1=DOWN';


/* =====================================================================
   SECTION 11 : MODEL REGISTRY
===================================================================== */

CREATE TABLE models (

    model_id SERIAL PRIMARY KEY,

    model_name VARCHAR(100) NOT NULL,

    algorithm VARCHAR(50) NOT NULL,

    version VARCHAR(50) NOT NULL,

    parameters JSONB,

    notes TEXT,

    created_at TIMESTAMPTZ DEFAULT NOW()
);


/* =====================================================================
   SECTION 12 : TRAINING RUNS
===================================================================== */

CREATE TABLE training_runs (

    run_id BIGSERIAL PRIMARY KEY,

    model_id INTEGER
        REFERENCES models(model_id),

    train_start TIMESTAMPTZ,

    train_end TIMESTAMPTZ,

    training_rows BIGINT,

    rmse DOUBLE PRECISION,

    mae DOUBLE PRECISION,

    accuracy DOUBLE PRECISION,

    f1_score DOUBLE PRECISION,

    created_at TIMESTAMPTZ DEFAULT NOW()
);


/* =====================================================================
   SECTION 13 : LIVE PREDICTIONS
===================================================================== */

CREATE TABLE predictions (

    prediction_id BIGSERIAL PRIMARY KEY,

    prediction_time TIMESTAMPTZ NOT NULL,

    model_id INTEGER
        REFERENCES models(model_id),

    asset_id INTEGER NOT NULL
        REFERENCES assets(asset_id),

    current_price DOUBLE PRECISION,

    predicted_return_10m DOUBLE PRECISION,

    predicted_price_10m DOUBLE PRECISION,

    predicted_direction SMALLINT,

    probability_up DOUBLE PRECISION,

    probability_down DOUBLE PRECISION,

    confidence DOUBLE PRECISION,

    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_prediction_asset_ts
ON predictions(asset_id, prediction_time DESC);


/* =====================================================================
   SECTION 14 : PREDICTION RESULTS
===================================================================== */

CREATE TABLE prediction_results (

    prediction_id BIGINT PRIMARY KEY
        REFERENCES predictions(prediction_id),

    actual_price DOUBLE PRECISION,

    actual_return DOUBLE PRECISION,

    prediction_error DOUBLE PRECISION,

    absolute_error DOUBLE PRECISION,

    correct_direction BOOLEAN,

    evaluated_at TIMESTAMPTZ DEFAULT NOW()
);


/* =====================================================================
   SECTION 15 : BACKTEST RESULTS
===================================================================== */

CREATE TABLE backtests (

    backtest_id BIGSERIAL PRIMARY KEY,

    model_id INTEGER
        REFERENCES models(model_id),

    asset_id INTEGER
        REFERENCES assets(asset_id),

    start_ts TIMESTAMPTZ,

    end_ts TIMESTAMPTZ,

    initial_balance DOUBLE PRECISION,

    final_balance DOUBLE PRECISION,

    pnl DOUBLE PRECISION,

    pnl_percent DOUBLE PRECISION,

    max_drawdown DOUBLE PRECISION,

    sharpe_ratio DOUBLE PRECISION,

    win_rate DOUBLE PRECISION,

    total_trades INTEGER,

    created_at TIMESTAMPTZ DEFAULT NOW()
);


/* =====================================================================
   SECTION 16 : TRADING SIGNALS
===================================================================== */

CREATE TABLE signals (

    signal_id BIGSERIAL PRIMARY KEY,

    signal_time TIMESTAMPTZ NOT NULL,

    model_id INTEGER
        REFERENCES models(model_id),

    asset_id INTEGER
        REFERENCES assets(asset_id),

    action VARCHAR(10),

    confidence DOUBLE PRECISION,

    entry_price DOUBLE PRECISION,

    stop_loss DOUBLE PRECISION,

    take_profit DOUBLE PRECISION,

    created_at TIMESTAMPTZ DEFAULT NOW()
);

/* =====================================================================
   SECTION 17 : CRYPTO OPTIONS MARKET DATA
===================================================================== */

CREATE TABLE options_instruments (
    option_id SERIAL PRIMARY KEY,
    asset_id INTEGER REFERENCES assets(asset_id),
    instrument_name VARCHAR(100) UNIQUE NOT NULL, -- e.g., "BTC-25DEC26-65000-C"
    strike_price NUMERIC(26,12) NOT NULL,
    expiration_time TIMESTAMPTZ NOT NULL,
    option_type CHAR(1) NOT NULL,                 -- 'C' = CALL, 'P' = PUT
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE options_snapshots (
    ts TIMESTAMPTZ NOT NULL,
    option_id INTEGER REFERENCES options_instruments(option_id),
    underlying_price DOUBLE PRECISION NOT NULL,
    bid_price DOUBLE PRECISION NOT NULL,
    ask_price DOUBLE PRECISION NOT NULL,
    volume_24h DOUBLE PRECISION,
    open_interest DOUBLE PRECISION,
    implied_volatility DOUBLE PRECISION,         -- Critical for ML prediction
    delta DOUBLE PRECISION,                      -- Option Greeks
    gamma DOUBLE PRECISION,
    theta DOUBLE PRECISION,
    vega DOUBLE PRECISION,
    PRIMARY KEY (ts, option_id)
);
 
    -- Optimize descending timeseries scans for options portfolio matrix reads
CREATE INDEX IF NOT EXISTS idx_options_snapshot_ts ON options_snapshots(option_id, ts DESC);
/* =====================================================================
   INITIAL ASSETS
===================================================================== */

INSERT INTO assets
(symbol, base_asset, quote_asset, asset_type)
VALUES
('BTCUSDT','BTC','USDT','SPOT'),
('ETHUSDT','ETH','USDT','SPOT'),
('BNBUSDT','BNB','USDT','SPOT'),
('ADAUSDT','ADA','USDT','SPOT'),
('SOLUSDT','SOL','USDT','SPOT'),
('UNIUSDT','UNI','USDT','SPOT');


/* =====================================================================
   INITIAL EXCHANGES
===================================================================== */

INSERT INTO exchanges(name)
VALUES
('Binance'),
('Bybit'),
('OKX'),
('Coinbase'),
('Kraken');


/* =====================================================================
   TIMESCALEDB OPTIONAL
===================================================================== */

/*
SELECT create_hypertable('ohlcv_1m', 'ts');
SELECT create_hypertable('trades', 'ts');
SELECT create_hypertable('orderbook_snapshot', 'ts');
SELECT create_hypertable('futures_metrics', 'ts');
SELECT create_hypertable('features_10m', 'ts');
SELECT create_hypertable('labels', 'ts');
SELECT create_hypertable('predictions', 'prediction_time');
*/