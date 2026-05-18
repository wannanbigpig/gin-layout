CREATE TABLE IF NOT EXISTS `upload_file_objects`
(
    `id`             int unsigned NOT NULL AUTO_INCREMENT,
    `storage_driver` varchar(20)  NOT NULL DEFAULT 'local' COMMENT '存储驱动:local,aliyun_oss,s3',
    `storage_base`   varchar(512) NOT NULL DEFAULT '' COMMENT '存储基础位置',
    `bucket`         varchar(128) NOT NULL DEFAULT '' COMMENT '存储桶',
    `storage_path`   varchar(512) NOT NULL DEFAULT '' COMMENT '实际存储路径',
    `object_key`     varchar(512) NOT NULL DEFAULT '' COMMENT '对象key',
    `size`           int unsigned NOT NULL DEFAULT '0' COMMENT '文件大小（字节）',
    `hash`           varchar(64)  NOT NULL DEFAULT '' COMMENT '文件SHA256哈希值',
    `mime_type`      varchar(100) NOT NULL DEFAULT '' COMMENT 'MIME类型',
    `etag`           varchar(128) NOT NULL DEFAULT '' COMMENT '对象ETag',
    `status`         varchar(32)  NOT NULL DEFAULT 'stored' COMMENT '物理对象状态:stored,delete_failed',
    `created_at`     datetime DEFAULT NULL COMMENT '创建时间',
    `updated_at`     datetime DEFAULT NULL COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uniq_driver_bucket_hash` (`storage_driver`, `bucket`, `hash`),
    KEY `idx_local_hash` (`storage_driver`, `hash`),
    KEY `idx_remote_bucket_hash` (`storage_driver`, `bucket`, `hash`),
    KEY `idx_status` (`status`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='上传文件物理对象表';

ALTER TABLE `upload_files`
    ADD COLUMN `file_object_id` int unsigned NOT NULL DEFAULT '0' COMMENT '物理对象ID' AFTER `id`,
    ADD KEY `idx_file_object_id` (`file_object_id`);
