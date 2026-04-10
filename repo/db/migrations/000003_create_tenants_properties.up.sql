-- Migration 000003: Tenant Profiles, Properties, Units, Staff Assignments, Technician Skills, Dispatch Cursors

CREATE TABLE IF NOT EXISTS properties (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL,
    address_line1 VARCHAR(255) NOT NULL,
    address_line2 VARCHAR(255) DEFAULT NULL,
    city VARCHAR(100) NOT NULL,
    state VARCHAR(50) NOT NULL,
    zip_code VARCHAR(20) NOT NULL,
    manager_id BIGINT UNSIGNED DEFAULT NULL,
    timezone VARCHAR(50) NOT NULL DEFAULT 'America/New_York',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    UNIQUE INDEX idx_properties_uuid (uuid),
    INDEX idx_properties_manager (manager_id),
    INDEX idx_properties_deleted_at (deleted_at),
    CONSTRAINT fk_properties_manager FOREIGN KEY (manager_id) REFERENCES users(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS units (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL,
    property_id BIGINT UNSIGNED NOT NULL,
    unit_number VARCHAR(50) NOT NULL,
    floor INT DEFAULT NULL,
    bedrooms INT DEFAULT NULL,
    bathrooms INT DEFAULT NULL,
    square_feet INT DEFAULT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'Vacant',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    UNIQUE INDEX idx_units_uuid (uuid),
    UNIQUE INDEX idx_units_property_number (property_id, unit_number),
    INDEX idx_units_status (status),
    INDEX idx_units_deleted_at (deleted_at),
    CONSTRAINT fk_units_property FOREIGN KEY (property_id) REFERENCES properties(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS tenant_profiles (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    unit_id BIGINT UNSIGNED DEFAULT NULL,
    emergency_contact_encrypted VARBINARY(512) DEFAULT NULL,
    emergency_contact_key_version INT DEFAULT NULL,
    lease_start DATE DEFAULT NULL,
    lease_end DATE DEFAULT NULL,
    move_in_date DATE DEFAULT NULL,
    notes TEXT DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    UNIQUE INDEX idx_tenant_profiles_uuid (uuid),
    UNIQUE INDEX idx_tenant_profiles_user (user_id),
    INDEX idx_tenant_profiles_unit (unit_id),
    INDEX idx_tenant_profiles_deleted_at (deleted_at),
    CONSTRAINT fk_tenant_profiles_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_tenant_profiles_unit FOREIGN KEY (unit_id) REFERENCES units(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS property_staff_assignments (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    property_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    role VARCHAR(50) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_staff_assignment_unique (property_id, user_id, role),
    CONSTRAINT fk_staff_property FOREIGN KEY (property_id) REFERENCES properties(id) ON DELETE CASCADE,
    CONSTRAINT fk_staff_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS technician_skill_tags (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    tag VARCHAR(100) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_tech_skill_unique (user_id, tag),
    INDEX idx_tech_skill_tag (tag),
    CONSTRAINT fk_tech_skill_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS dispatch_cursors (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    property_id BIGINT UNSIGNED NOT NULL,
    skill_tag VARCHAR(100) NOT NULL,
    last_assigned_user_id BIGINT UNSIGNED DEFAULT NULL,
    cursor_position INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_dispatch_cursor_unique (property_id, skill_tag),
    CONSTRAINT fk_dispatch_cursor_property FOREIGN KEY (property_id) REFERENCES properties(id) ON DELETE CASCADE,
    CONSTRAINT fk_dispatch_cursor_user FOREIGN KEY (last_assigned_user_id) REFERENCES users(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
