-- Migration 000009: Saved Reports, Generated Reports

CREATE TABLE IF NOT EXISTS saved_reports (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL,
    report_type VARCHAR(50) NOT NULL,
    filters JSON NOT NULL,
    output_format VARCHAR(20) NOT NULL DEFAULT 'CSV',
    schedule VARCHAR(100) DEFAULT NULL,
    owner_id BIGINT UNSIGNED NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_generated_at TIMESTAMP NULL DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_saved_reports_uuid (uuid),
    INDEX idx_saved_reports_owner (owner_id),
    INDEX idx_saved_reports_type (report_type),
    CONSTRAINT fk_saved_reports_owner FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS generated_reports (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL,
    saved_report_id BIGINT UNSIGNED DEFAULT NULL,
    report_type VARCHAR(50) NOT NULL,
    format VARCHAR(20) NOT NULL DEFAULT 'CSV',
    storage_path VARCHAR(512) NOT NULL,
    file_size BIGINT UNSIGNED NOT NULL DEFAULT 0,
    record_count INT NOT NULL DEFAULT 0,
    parameters JSON DEFAULT NULL,
    generated_by BIGINT UNSIGNED NOT NULL,
    generated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_generated_reports_uuid (uuid),
    INDEX idx_generated_reports_saved (saved_report_id),
    INDEX idx_generated_reports_type (report_type),
    INDEX idx_generated_reports_date (generated_at),
    CONSTRAINT fk_generated_reports_saved FOREIGN KEY (saved_report_id) REFERENCES saved_reports(id) ON DELETE SET NULL,
    CONSTRAINT fk_generated_reports_by FOREIGN KEY (generated_by) REFERENCES users(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
