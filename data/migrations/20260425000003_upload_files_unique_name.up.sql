-- 为 upload_files 增加「同目录同名唯一」约束，消除并发上传的 TOCTOU 竞态。
-- 范式对齐 upload_file_folders.uniq_parent_name_deleted：deleted_at 纳入唯一键，
-- 使软删记录不占用活跃命名空间（软删后 deleted_at=时间戳，与活跃记录 deleted_at=0 不冲突）。
ALTER TABLE `upload_files`
    ADD UNIQUE KEY `uniq_folder_name_deleted` (`folder_id`, `origin_name`, `deleted_at`);
