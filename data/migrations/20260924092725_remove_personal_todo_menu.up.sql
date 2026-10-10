BEGIN;

-- 该版本在初始化时可能已按占位内容执行，保留为空迁移以保持版本历史一致。
-- 实际菜单调整由后续 remove_personal_todo_menu_record 迁移执行。

COMMIT;
