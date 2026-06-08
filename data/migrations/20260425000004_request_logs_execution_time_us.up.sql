ALTER TABLE `request_logs`
    ADD COLUMN `execution_time_us` bigint NOT NULL DEFAULT '0' COMMENT '执行时间（微秒，统一单位）' AFTER `response_header`;

UPDATE `request_logs`
SET `execution_time_us` = CASE
    WHEN `execution_time_unit` = 's' THEN CAST(ROUND(`execution_time` * 1000000) AS SIGNED)
    ELSE CAST(ROUND(`execution_time` * 1000) AS SIGNED)
END;

ALTER TABLE `request_logs`
    DROP COLUMN `execution_time`,
    DROP COLUMN `execution_time_unit`;
