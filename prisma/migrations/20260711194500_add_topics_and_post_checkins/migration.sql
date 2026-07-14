-- CreateTable
CREATE TABLE `topics` (
    `id` INTEGER NOT NULL AUTO_INCREMENT,
    `slug` VARCHAR(80) NOT NULL,
    `title` VARCHAR(50) NOT NULL,
    `description` VARCHAR(500) NULL,
    `status` INTEGER NOT NULL DEFAULT 1,
    `sort` INTEGER NOT NULL DEFAULT 0,
    `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    `updated_at` DATETIME(3) NOT NULL,

    UNIQUE INDEX `topics_slug_key`(`slug`),
    UNIQUE INDEX `topics_title_key`(`title`),
    PRIMARY KEY (`id`)
) DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- Seed the system topics inside the migration so the default topic is available
-- before any application instance starts accepting new posts.
INSERT INTO `topics` (`slug`, `title`, `description`, `status`, `sort`, `created_at`, `updated_at`)
VALUES
    ('weekly-reading', '本周精读', '拆解一本书的关键观点', 1, 30, CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('check-in-challenge', '打卡挑战', '记录每天的阅读与成长', 1, 100, CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('course-resources', '课程资料', '文档、视频、回放统一收纳', 1, 20, CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3))
ON DUPLICATE KEY UPDATE
    `title` = VALUES(`title`),
    `description` = VALUES(`description`),
    `status` = VALUES(`status`),
    `sort` = VALUES(`sort`),
    `updated_at` = CURRENT_TIMESTAMP(3);

-- AlterTable
-- topic_id remains nullable so historical posts can be migrated without inventing
-- user choices. The application service always resolves a topic for new posts.
ALTER TABLE `posts` ADD COLUMN `topic_id` INTEGER NULL;

-- AlterTable
-- Historical standalone check-ins keep NULL values. All new check-ins receive a
-- post_id and an Asia/Shanghai YYYY-MM-DD check_in_date.
ALTER TABLE `check_ins`
    ADD COLUMN `post_id` INTEGER NULL,
    ADD COLUMN `check_in_date` CHAR(10) NULL;

-- CreateIndex
CREATE INDEX `posts_topic_id_created_at_idx` ON `posts`(`topic_id`, `created_at`);

-- CreateIndex
CREATE UNIQUE INDEX `check_ins_post_id_key` ON `check_ins`(`post_id`);

-- CreateIndex
-- MySQL permits multiple NULL values in a unique index, which preserves legacy
-- rows while enforcing at most one new check-in per user and business date.
CREATE UNIQUE INDEX `check_ins_user_id_check_in_date_key` ON `check_ins`(`user_id`, `check_in_date`);

-- CreateIndex
CREATE INDEX `check_ins_user_id_created_at_idx` ON `check_ins`(`user_id`, `created_at`);

-- AddForeignKey
ALTER TABLE `posts` ADD CONSTRAINT `posts_topic_id_fkey`
    FOREIGN KEY (`topic_id`) REFERENCES `topics`(`id`)
    ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE `check_ins` ADD CONSTRAINT `check_ins_post_id_fkey`
    FOREIGN KEY (`post_id`) REFERENCES `posts`(`id`)
    ON DELETE SET NULL ON UPDATE CASCADE;
