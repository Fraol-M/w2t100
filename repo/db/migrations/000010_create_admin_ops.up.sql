-- Migration 000010: Audit Logs, Encryption Key Versions, System Settings

CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL,
    actor_id BIGINT UNSIGNED DEFAULT NULL,
    action VARCHAR(50) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id BIGINT UNSIGNED DEFAULT NULL,
    description TEXT DEFAULT NULL,
    old_values JSON DEFAULT NULL,
    new_values JSON DEFAULT NULL,
    ip_address VARCHAR(45) DEFAULT NULL,
    request_id VARCHAR(36) DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_audit_logs_uuid (uuid),
    INDEX idx_audit_logs_resource (resource_type, resource_id),
    INDEX idx_audit_logs_actor (actor_id),
    INDEX idx_audit_logs_action (action),
    INDEX idx_audit_logs_created (created_at),
    INDEX idx_audit_logs_request (request_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- encryption_key_versions tracks which key versions exist and which is active.
-- Actual key material is stored as files in ENCRYPTION_KEY_DIR; this table is an
-- audit registry updated on each key rotation via POST /admin/keys/rotate.
CREATE TABLE IF NOT EXISTS encryption_key_versions (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    key_version INT NOT NULL,
    algorithm VARCHAR(50) NOT NULL DEFAULT 'AES-256-GCM',
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    activated_at TIMESTAMP NULL DEFAULT NULL,
    rotated_at TIMESTAMP NULL DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_key_versions_version (key_version),
    INDEX idx_key_versions_active (is_active)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS system_settings (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    setting_key VARCHAR(255) NOT NULL,
    setting_value TEXT NOT NULL,
    description VARCHAR(500) DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_system_settings_key (setting_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
