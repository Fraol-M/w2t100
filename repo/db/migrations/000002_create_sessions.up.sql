-- Migration 000002: Sessions
-- DB-backed opaque session management with idle and absolute expiry.

CREATE TABLE IF NOT EXISTS sessions (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    token_hash VARCHAR(128) NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    ip_address VARCHAR(45) DEFAULT NULL,
    user_agent VARCHAR(512) DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_active_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    idle_expires_at TIMESTAMP NOT NULL,
    absolute_expires_at TIMESTAMP NOT NULL,
    revoked_at TIMESTAMP NULL DEFAULT NULL,
    UNIQUE INDEX idx_sessions_token_hash (token_hash),
    INDEX idx_sessions_user_id (user_id),
    INDEX idx_sessions_expires (idle_expires_at, absolute_expires_at),
    CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
