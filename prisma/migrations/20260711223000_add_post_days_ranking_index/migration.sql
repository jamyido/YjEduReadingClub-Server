-- Accelerate per-circle cumulative post-day ranking queries.
CREATE INDEX `posts_circle_id_status_author_id_created_at_idx`
ON `posts`(`circle_id`, `status`, `author_id`, `created_at`);
