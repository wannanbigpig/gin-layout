ALTER TABLE `menu`
    ADD COLUMN `active_code_unique` varchar(120)
        GENERATED ALWAYS AS (CASE WHEN `deleted_at` = 0 AND `code` <> '' THEN `code` ELSE NULL END) STORED,
    ADD COLUMN `active_name_unique` varchar(255)
        GENERATED ALWAYS AS (CASE WHEN `deleted_at` = 0 AND `name` <> '' THEN `name` ELSE NULL END) STORED,
    ADD COLUMN `active_full_path_unique` varchar(255)
        GENERATED ALWAYS AS (CASE WHEN `deleted_at` = 0 AND `full_path` <> '' THEN `full_path` ELSE NULL END) STORED,
    ADD UNIQUE KEY `uniq_menu_code_active` (`active_code_unique`),
    ADD UNIQUE KEY `uniq_menu_name_active` (`active_name_unique`),
    ADD UNIQUE KEY `uniq_menu_full_path_active` (`active_full_path_unique`);
