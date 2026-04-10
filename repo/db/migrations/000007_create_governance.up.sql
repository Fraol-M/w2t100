-- Migration 000007: Governance — Reports, Enforcement Actions, Keywords Blacklist, Risk Rules

CREATE TABLE IF NOT EXISTS reports (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL,
    reporter_id BIGINT UNSIGNED NOT NULL,
    target_type VARCHAR(30) NOT NULL,
    target_id BIGINT UNSIGNED NOT NULL,
    category VARCHAR(50) NOT NULL,
    description TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'Open',
    reviewer_id BIGINT UNSIGNED DEFAULT NULL,
    resolution_notes TEXT DEFAULT NULL,
    resolved_at TIMESTAMP NULL DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_reports_uuid (uuid),
    INDEX idx_reports_target (target_type, target_id),
    INDEX idx_reports_status (status),
    INDEX idx_reports_reporter (reporter_id),
    INDEX idx_reports_reviewer (reviewer_id),
    CONSTRAINT fk_reports_reporter FOREIGN KEY (reporter_id) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT fk_reports_reviewer FOREIGN KEY (reviewer_id) REFERENCES users(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS enforcement_actions (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    report_id BIGINT UNSIGNED DEFAULT NULL,
    action_type VARCHAR(30) NOT NULL,
    reason TEXT NOT NULL,
    starts_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ends_at TIMESTAMP NULL DEFAULT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    rate_limit_max INT DEFAULT NULL,
    rate_limit_window_minutes INT DEFAULT NULL,
    created_by BIGINT UNSIGNED NOT NULL,
    revoked_at TIMESTAMP NULL DEFAULT NULL,
    revoked_by BIGINT UNSIGNED DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_enforcement_uuid (uuid),
    INDEX idx_enforcement_user_active (user_id, is_active),
    INDEX idx_enforcement_type (action_type),
    CONSTRAINT fk_enforcement_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_enforcement_report FOREIGN KEY (report_id) REFERENCES reports(id) ON DELETE SET NULL,
    CONSTRAINT fk_enforcement_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT fk_enforcement_revoked_by FOREIGN KEY (revoked_by) REFERENCES users(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS keywords_blacklist (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    keyword VARCHAR(255) NOT NULL,
    category VARCHAR(50) NOT NULL DEFAULT 'General',
    severity VARCHAR(20) NOT NULL DEFAULT 'Medium',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_keywords_keyword (keyword),
    INDEX idx_keywords_active (is_active)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS risk_rules (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT DEFAULT NULL,
    condition_type VARCHAR(50) NOT NULL,
    condition_params JSON NOT NULL,
    action_type VARCHAR(30) NOT NULL,
    action_params JSON DEFAULT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_risk_rules_uuid (uuid),
    UNIQUE INDEX idx_risk_rules_name (name),
    INDEX idx_risk_rules_active (is_active)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
