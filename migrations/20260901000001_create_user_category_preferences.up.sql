CREATE TABLE IF NOT EXISTS user_category_preferences (
    user_id    VARCHAR(64) NOT NULL,
    category   VARCHAR(64) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, category)
);

CREATE INDEX IF NOT EXISTS idx_user_category_preferences_category ON user_category_preferences(category);
