-- Migration 000005: Attachments
-- Stores file metadata for work order images and report evidence.

CREATE TABLE IF NOT EXISTS attachments (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL,
    entity_type VARCHAR(50) NOT NULL DEFAULT 'WorkOrder',
    entity_id BIGINT UNSIGNED NOT NULL,
    filename VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    file_size BIGINT UNSIGNED NOT NULL,
    sha256_hash VARCHAR(64) NOT NULL,
    storage_path VARCHAR(512) NOT NULL,
    uploaded_by BIGINT UNSIGNED NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_attachments_uuid (uuid),
    INDEX idx_attachments_entity (entity_type, entity_id),
    INDEX idx_attachments_hash (sha256_hash),
    CONSTRAINT fk_attachments_uploaded_by FOREIGN KEY (uploaded_by) REFERENCES users(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
