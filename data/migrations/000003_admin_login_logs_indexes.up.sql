ALTER TABLE `admin_login_logs`
    ADD KEY `aall_uid_deleted_at_is_revoked_login_status_token_expires` (`uid`, `deleted_at`, `is_revoked`, `login_status`, `token_expires`),
    ADD KEY `aall_jwt_id_deleted_at_is_revoked` (`jwt_id`, `deleted_at`, `is_revoked`);
