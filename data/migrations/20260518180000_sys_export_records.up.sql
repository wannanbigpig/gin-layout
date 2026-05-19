START TRANSACTION;

CREATE TABLE IF NOT EXISTS `sys_export_records`
(
    `id`              bigint unsigned NOT NULL AUTO_INCREMENT,
    `scene`           varchar(60)     NOT NULL DEFAULT '' COMMENT '导出场景',
    `task_run_id`     bigint unsigned NOT NULL DEFAULT '0' COMMENT '任务执行记录ID',
    `trigger_user_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '触发人ID',
    `status`          varchar(20)     NOT NULL DEFAULT '' COMMENT '导出状态 pending/running/success/failed/canceled/retrying',
    `queue`           varchar(60)     NOT NULL DEFAULT '' COMMENT '队列名称',
    `progress`        tinyint unsigned NOT NULL DEFAULT '0' COMMENT '进度 0-100',
    `total_count`     bigint unsigned NOT NULL DEFAULT '0' COMMENT '本次导出总记录数',
    `processed_count` bigint unsigned NOT NULL DEFAULT '0' COMMENT '本次导出已处理记录数',
    `file_id`         bigint unsigned NOT NULL DEFAULT '0' COMMENT '导出文件ID',
    `file_uuid`       varchar(64)     NOT NULL DEFAULT '' COMMENT '导出文件UUID',
    `file_name`       varchar(255)    NOT NULL DEFAULT '' COMMENT '导出文件名',
    `file_size`       bigint unsigned NOT NULL DEFAULT '0' COMMENT '导出文件大小',
    `format`          varchar(30)     NOT NULL DEFAULT '' COMMENT '导出格式',
    `error_message`   text                     DEFAULT NULL COMMENT '错误信息',
    `started_at`      datetime                 DEFAULT NULL COMMENT '开始时间',
    `finished_at`     datetime                 DEFAULT NULL COMMENT '结束时间',
    `created_at`      datetime                 DEFAULT NULL,
    `updated_at`      datetime                 DEFAULT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `ser_task_run_id` (`task_run_id`),
    KEY `ser_scene_created_at` (`scene`, `created_at`),
    KEY `ser_status_created_at` (`status`, `created_at`),
    KEY `ser_trigger_user_id_created_at` (`trigger_user_id`, `created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='统一导出记录表';

COMMIT;
