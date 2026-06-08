ALTER TABLE `request_logs`
    ADD COLUMN `execution_time` DECIMAL(12,4) NOT NULL DEFAULT '0.0000' COMMENT '执行时间（毫秒，支持小数，最多4位）' AFTER `response_header`,
    ADD COLUMN `execution_time_unit` varchar(16) NOT NULL DEFAULT 'ms' COMMENT '执行时间单位：ms' AFTER `execution_time`;

UPDATE `request_logs`
SET `execution_time` = `execution_time_us` / 1000.0,
    `execution_time_unit` = 'ms';

ALTER TABLE `request_logs`
    DROP COLUMN `execution_time_us`;
