ALTER TABLE `admin_login_logs`
    DROP KEY `aall_uid_deleted_at_is_revoked_login_status_token_expires`,
    DROP KEY `aall_jwt_id_deleted_at_is_revoked`;
