-- Старая цена: заполняется при изменении цены, чтобы показать «было/стало» в сообщении.
ALTER TABLE lots ADD COLUMN IF NOT EXISTS previous_price VARCHAR(50);

-- ID сообщений, отправленных в Telegram по каждому лоту (для последующего удаления).
CREATE TABLE IF NOT EXISTS sent_messages (
    id         BIGSERIAL PRIMARY KEY,
    lot_id     VARCHAR(50) NOT NULL REFERENCES lots(id) ON DELETE CASCADE,
    chat_id    BIGINT NOT NULL,
    message_id INT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sent_messages_lot_id ON sent_messages(lot_id);
