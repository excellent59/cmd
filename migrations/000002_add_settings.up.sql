CREATE TABLE IF NOT EXISTS settings (
    id                    INT PRIMARY KEY DEFAULT 1,
    poll_interval_seconds INT NOT NULL,
    categories            TEXT NOT NULL DEFAULT '',
    updated_at            TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT settings_singleton CHECK (id = 1)
);

CREATE INDEX IF NOT EXISTS idx_lots_is_sent ON lots(is_sent);
