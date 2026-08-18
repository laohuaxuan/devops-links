-- DevOps 链接导航平台

CREATE TABLE IF NOT EXISTS `users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `name` varchar(64) NOT NULL,
  `display_name` varchar(120) DEFAULT NULL,
  `email` varchar(120) DEFAULT NULL,
  `password_hash` varchar(255) DEFAULT NULL,
  `password_changed_at` datetime(3) DEFAULT NULL,
  `auth_source` varchar(20) DEFAULT 'local',
  `role` varchar(16) NOT NULL,
  `status` varchar(20) DEFAULT 'active',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_users_name` (`name`),
  UNIQUE KEY `idx_users_email` (`email`),
  KEY `idx_users_role` (`role`),
  KEY `idx_users_auth_source` (`auth_source`),
  KEY `idx_users_status` (`status`),
  KEY `idx_users_password_changed_at` (`password_changed_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `reset_tokens` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `user_id` bigint unsigned NOT NULL,
  `token` varchar(64) NOT NULL,
  `expires_at` bigint NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_reset_tokens_user_id` (`user_id`),
  UNIQUE KEY `idx_reset_tokens_token` (`token`),
  KEY `idx_reset_tokens_expires_at` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `password_reset_request_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `user_id` bigint unsigned DEFAULT NULL,
  `email` varchar(120) DEFAULT NULL,
  `ip` varchar(64) DEFAULT NULL,
  `device` varchar(512) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_password_reset_request_logs_user_id` (`user_id`),
  KEY `idx_password_reset_request_logs_email` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `audit_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `user_id` bigint unsigned DEFAULT NULL,
  `username` varchar(100) DEFAULT NULL,
  `display_name` varchar(120) DEFAULT NULL,
  `action` varchar(60) DEFAULT NULL,
  `result` varchar(20) DEFAULT NULL,
  `ip` varchar(64) DEFAULT NULL,
  `detail` varchar(1000) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_audit_logs_user_id` (`user_id`),
  KEY `idx_audit_logs_username` (`username`),
  KEY `idx_audit_logs_display_name` (`display_name`),
  KEY `idx_audit_logs_action` (`action`),
  KEY `idx_audit_logs_result` (`result`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `categories` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `parent_id` bigint unsigned DEFAULT 0,
  `name` varchar(128) NOT NULL,
  `scope` varchar(16) NOT NULL DEFAULT 'shared',
  `owner_id` bigint unsigned NOT NULL DEFAULT 0,
  `sort_order` int DEFAULT 0,
  `created_by` bigint unsigned DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_category_scope_owner_parent_name` (`scope`, `owner_id`, `parent_id`, `name`),
  KEY `idx_categories_parent_id` (`parent_id`),
  KEY `idx_categories_scope` (`scope`),
  KEY `idx_categories_owner_id` (`owner_id`),
  KEY `idx_categories_created_by` (`created_by`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `links` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `category_id` bigint unsigned NOT NULL,
  `name` varchar(128) NOT NULL,
  `url` varchar(512) NOT NULL,
  `icon_path` varchar(512) DEFAULT NULL,
  `maintainer` varchar(128) DEFAULT NULL,
  `remark` text,
  `sort_order` int DEFAULT 0,
  `created_by` bigint unsigned DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_links_category_id` (`category_id`),
  KEY `idx_links_created_by` (`created_by`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
