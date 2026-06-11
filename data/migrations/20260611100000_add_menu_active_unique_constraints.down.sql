ALTER TABLE `menu`
    DROP INDEX `uniq_menu_full_path_active`,
    DROP INDEX `uniq_menu_name_active`,
    DROP INDEX `uniq_menu_code_active`,
    DROP COLUMN `active_full_path_unique`,
    DROP COLUMN `active_name_unique`,
    DROP COLUMN `active_code_unique`;
