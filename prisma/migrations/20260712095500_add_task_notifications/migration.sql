-- Extend the notification enum without changing the ordinal of existing values.
ALTER TABLE `notifications`
  MODIFY `type` ENUM('LIKE', 'COMMENT', 'FOLLOW', 'SYSTEM', 'CIRCLE_INVITE', 'TASK') NOT NULL;

-- Normalize legacy task-like system notifications to the dedicated type.
UPDATE `notifications`
SET `type` = 'TASK'
WHERE `type` = 'SYSTEM'
  AND LOWER(`target_type`) = 'task';

-- Support unread inbox and summary queries by recipient and creation time.
CREATE INDEX `notifications_user_id_is_read_created_at_idx`
  ON `notifications`(`user_id`, `is_read`, `created_at`);
