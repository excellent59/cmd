CREATE TABLE IF NOT EXISTS lots (
    id VARCHAR(50) PRIMARY KEY,
    url TEXT NOT NULL,
    title TEXT NOT NULL,
    year INT,
    current_price VARCHAR(50) NOT NULL,
    location VARCHAR(200),
    price_trend VARCHAR(10),
    images JSONB DEFAULT '[]',
    is_sent BOOLEAN DEFAULT FALSE, 
    last_checked_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS price_histories (
    id SERIAL PRIMARY KEY,
    lot_id VARCHAR(50) NOT NULL REFERENCES lots(id) ON DELETE CASCADE,
    old_price VARCHAR(50) NOT NULL,
    new_price VARCHAR(50) NOT NULL,
    changed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_lots_price ON lots(current_price);
CREATE INDEX IF NOT EXISTS idx_histories_lot_id ON price_histories(lot_id);
CREATE INDEX IF NOT EXISTS idx_histories_changed_at ON price_histories(changed_at);