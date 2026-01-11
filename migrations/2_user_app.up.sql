CREATE TABLE IF NOT EXISTS user_app
(
    id      INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL,
    app_id  INTEGER NOT NULL,
    is_enabled BOOLEAN NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE,
    UNIQUE(user_id, app_id)
);

CREATE INDEX IF NOT EXISTS idx_user_app_user_id ON user_app (user_id);
CREATE INDEX IF NOT EXISTS idx_user_app_app_id ON user_app (app_id);