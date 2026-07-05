/* ============================================================================
   SUPPLEMENTARY MARKET DATA SCHEMA
   File: 002_supplementary_market_data.sql

   PURPOSE
   ----------------------------------------------------------------------------
   This schema stores external signals that may influence crypto prices.

   These datasets typically include:

   - Commodities
   - Equity indices
   - Currency indices
   - Treasury yields
   - Federal Reserve rates
   - Economic indicators
   - Economic events
   - Sentiment indicators
   - Custom alternative datasets

   The objective is to enrich ML training datasets and improve
   prediction performance.

   Examples:

   Commodities:
     XAUUSD (Gold)
     XAGUSD (Silver)
     WTI
     BRENT
     NATGAS

   Market Indices:
     SPX
     SPY
     QQQ
     DXY
     VIX

   Interest Rates:
     FEDFUNDS
     SOFR
     US02Y
     US10Y
     US30Y

   Economic Indicators:
     CPI
     CORE_CPI
     GDP
     NFP
     UNEMPLOYMENT

   Sentiment:
     Fear & Greed Index
     News Sentiment
     Social Sentiment

   ========================================================================= */


/* ============================================================================
   SECTION 1
   DATA PROVIDERS
   ============================================================================

   Purpose:
   Tracks where data originates.

   Examples:
   - FRED
   - TradingEconomics
   - AlphaVantage
   - Yahoo Finance
   - Polygon
   - CoinGlass
   - Glassnode
   - Alternative.me
*/

CREATE TABLE data_sources (

    source_id SERIAL PRIMARY KEY,

    source_name VARCHAR(100) NOT NULL UNIQUE,

    source_type VARCHAR(50),

    api_endpoint TEXT,

    provider_website TEXT,

    description TEXT,

    is_active BOOLEAN DEFAULT TRUE,

    created_at TIMESTAMPTZ DEFAULT NOW()
);



/* ============================================================================
   SECTION 2
   EXTERNAL ASSET DEFINITIONS
   ============================================================================

   Purpose:
   Defines every supplementary asset.

   Categories:

   COMMODITY
   INDEX
   RATE
   ECONOMIC
   SENTIMENT
   CUSTOM

   Examples:

   XAUUSD
   DXY
   VIX
   FEDFUNDS
   CPI
*/

CREATE TABLE external_assets (

    external_asset_id SERIAL PRIMARY KEY,

    symbol VARCHAR(50) NOT NULL UNIQUE,

    asset_name VARCHAR(200) NOT NULL,

    category VARCHAR(50) NOT NULL,

    unit VARCHAR(50),

    source_id INTEGER
        REFERENCES data_sources(source_id),

    fetch_interval VARCHAR(20),

    description TEXT,

    is_active BOOLEAN DEFAULT TRUE,

    created_at TIMESTAMPTZ DEFAULT NOW()
);



/* ============================================================================
   SECTION 3
   EXTERNAL TIMESERIES VALUES
   ============================================================================

   Purpose:
   Generic storage for all supplementary data.

   Examples:

   DXY:
     108.43

   Gold:
     3450.20

   Fed Funds:
     4.50

   CPI:
     2.90

   This design allows any future indicator without schema changes.
*/

CREATE TABLE external_timeseries (

    ts TIMESTAMPTZ NOT NULL,

    external_asset_id INTEGER NOT NULL
        REFERENCES external_assets(external_asset_id),

    value DOUBLE PRECISION NOT NULL,

    frequency VARCHAR(20),

    source_reference TEXT,

    notes TEXT,

    created_at TIMESTAMPTZ DEFAULT NOW(),

    PRIMARY KEY (
        ts,
        external_asset_id
    )
);

CREATE INDEX idx_external_timeseries_asset_time
ON external_timeseries (
    external_asset_id,
    ts DESC
);



/* ============================================================================
   SECTION 4
   ECONOMIC EVENT TYPES
   ============================================================================

   Purpose:
   Maintain master list of economic events.

   Examples:

   CPI
   PPI
   GDP
   NFP
   FOMC
   UNEMPLOYMENT
*/

CREATE TABLE economic_event_types (

    event_type_id SERIAL PRIMARY KEY,

    event_name VARCHAR(200) UNIQUE NOT NULL,

    country_code VARCHAR(20),

    category VARCHAR(100),

    description TEXT,

    default_importance SMALLINT,

    created_at TIMESTAMPTZ DEFAULT NOW()
);



/* ============================================================================
   SECTION 5
   ECONOMIC EVENT CALENDAR
   ============================================================================

   Purpose:
   Stores actual economic releases.

   Examples:

   US CPI
   US GDP
   US NFP
   ECB Rate Decision
*/

CREATE TABLE economic_events (

    event_id BIGSERIAL PRIMARY KEY,

    event_type_id INTEGER NOT NULL
        REFERENCES economic_event_types(event_type_id),

    event_time TIMESTAMPTZ NOT NULL,

    country_code VARCHAR(20),

    importance SMALLINT,

    actual DOUBLE PRECISION,

    forecast DOUBLE PRECISION,

    previous DOUBLE PRECISION,

    unit VARCHAR(50),

    surprise DOUBLE PRECISION,

    notes TEXT,

    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_economic_events_time
ON economic_events(event_time DESC);



/* ============================================================================
   SECTION 6
   FEAR & GREED INDEX
   ============================================================================

   Purpose:
   Stores historical Fear & Greed values.

   Typical Range:
   0 - 100

   Interpretation:

   0-24   Extreme Fear
   25-49  Fear
   50     Neutral
   51-74  Greed
   75-100 Extreme Greed
*/

CREATE TABLE fear_greed_index (

    ts TIMESTAMPTZ PRIMARY KEY,

    score DOUBLE PRECISION NOT NULL,

    classification VARCHAR(50),

    source_reference TEXT,

    created_at TIMESTAMPTZ DEFAULT NOW()
);



/* ============================================================================
   SECTION 7
   NEWS SENTIMENT
   ============================================================================

   Purpose:
   Aggregated sentiment from financial media.

   Range Example:
   -1.0 -> Very Bearish
    0.0 -> Neutral
   +1.0 -> Very Bullish
*/

CREATE TABLE news_sentiment (

    ts TIMESTAMPTZ PRIMARY KEY,

    sentiment_score DOUBLE PRECISION,

    article_count INTEGER,

    positive_articles INTEGER,

    neutral_articles INTEGER,

    negative_articles INTEGER,

    source_reference TEXT,

    created_at TIMESTAMPTZ DEFAULT NOW()
);



/* ============================================================================
   SECTION 8
   SOCIAL SENTIMENT
   ============================================================================

   Purpose:
   Tracks sentiment extracted from social platforms.

   Examples:
   - X/Twitter
   - Reddit
   - Telegram
   - Discord
*/

CREATE TABLE social_sentiment (

    ts TIMESTAMPTZ PRIMARY KEY,

    sentiment_score DOUBLE PRECISION,

    mention_count BIGINT,

    engagement_score DOUBLE PRECISION,

    positive_mentions BIGINT,

    neutral_mentions BIGINT,

    negative_mentions BIGINT,

    source_reference TEXT,

    created_at TIMESTAMPTZ DEFAULT NOW()
);



/* ============================================================================
   SECTION 9
   FEATURE SNAPSHOTS
   ============================================================================

   Purpose:
   Materialized supplementary features.

   This allows preprocessing once and reusing
   across all prediction models.

   Example Feature Set:

   {
      "gold_price": 3450.5,
      "gold_return_1d": 0.52,
      "oil_price": 77.2,
      "dxy": 108.1,
      "vix": 18.4,
      "us10y": 4.34,
      "fear_greed": 68
   }
*/

CREATE TABLE supplementary_features (

    ts TIMESTAMPTZ NOT NULL,

    timeframe VARCHAR(20) NOT NULL,

    feature_json JSONB NOT NULL,

    feature_version VARCHAR(50),

    created_at TIMESTAMPTZ DEFAULT NOW(),

    PRIMARY KEY (
        ts,
        timeframe
    )
);



/* ============================================================================
   SECTION 10
   FEATURE GENERATION LOG
   ============================================================================

   Purpose:
   Audit trail for generated supplementary features.
*/

CREATE TABLE feature_generation_runs (

    run_id BIGSERIAL PRIMARY KEY,

    feature_version VARCHAR(50),

    generated_rows BIGINT,

    generation_start TIMESTAMPTZ,

    generation_end TIMESTAMPTZ,

    status VARCHAR(50),

    notes TEXT,

    created_at TIMESTAMPTZ DEFAULT NOW()
);



/* ============================================================================
   SECTION 11
   DATA COLLECTION CONFIGURATION
   ============================================================================

   Purpose:
   Stores runtime collection configuration.

   Example:

   Symbol:
   DXY

   Frequency:
   15m

   Provider:
   FRED
*/

CREATE TABLE collection_configurations (

    config_id BIGSERIAL PRIMARY KEY,

    external_asset_id INTEGER
        REFERENCES external_assets(external_asset_id),

    enabled BOOLEAN DEFAULT TRUE,

    fetch_interval VARCHAR(20),

    retention_days INTEGER,

    priority SMALLINT,

    notes TEXT,

    created_at TIMESTAMPTZ DEFAULT NOW(),

    updated_at TIMESTAMPTZ DEFAULT NOW()
);



/* ============================================================================
   SECTION 12
   COLLECTION EXECUTION LOG
   ============================================================================

   Purpose:
   Monitor ingestion jobs.
*/

CREATE TABLE collection_runs (

    run_id BIGSERIAL PRIMARY KEY,

    external_asset_id INTEGER
        REFERENCES external_assets(external_asset_id),

    started_at TIMESTAMPTZ,

    finished_at TIMESTAMPTZ,

    records_inserted INTEGER,

    records_updated INTEGER,

    status VARCHAR(50),

    error_message TEXT,

    created_at TIMESTAMPTZ DEFAULT NOW()
);



/* ============================================================================
   RECOMMENDED INITIAL RECORDS
   ============================================================================

   Recommended External Assets

   Commodities:
     XAUUSD
     XAGUSD
     WTI
     BRENT
     NATGAS

   Market:
     DXY
     VIX
     SPX
     QQQ

   Rates:
     FEDFUNDS
     SOFR
     US02Y
     US10Y
     US30Y

   Economic:
     CPI
     CORE_CPI
     PPI
     GDP
     NFP
     UNEMPLOYMENT

   Sentiment:
     FEAR_GREED
     NEWS_SENTIMENT
     SOCIAL_SENTIMENT
*/


/* ============================================================================
   TIMESCALEDB (OPTIONAL)
   ============================================================================

SELECT create_hypertable('external_timeseries', 'ts');

SELECT create_hypertable('economic_events', 'event_time');

SELECT create_hypertable('fear_greed_index', 'ts');

SELECT create_hypertable('news_sentiment', 'ts');

SELECT create_hypertable('social_sentiment', 'ts');

SELECT create_hypertable('supplementary_features', 'ts');

*/
