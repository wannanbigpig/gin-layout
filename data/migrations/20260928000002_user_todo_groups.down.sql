ALTER TABLE `user_todos`
    DROP KEY `ut_assignee_group_deleted`,
    DROP COLUMN `group_id`;

DROP TABLE IF EXISTS `user_todo_groups`;
