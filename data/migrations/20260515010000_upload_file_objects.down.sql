ALTER TABLE `upload_files`
    DROP KEY `idx_file_object_id`,
    DROP COLUMN `file_object_id`;

DROP TABLE IF EXISTS `upload_file_objects`;
