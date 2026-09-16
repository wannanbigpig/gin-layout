ALTER TABLE `admin_login_logs`
    ADD INDEX `idx_refresh_session` (`refresh_token_hash`, `deleted_at`, `is_revoked`, `login_status`);
