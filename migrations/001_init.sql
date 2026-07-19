-- 嘉阅圈后端数据库初始化脚本（PostgreSQL 版本）
-- 对应原 Prisma schema 的最终状态（合并所有历史迁移后的完整表结构）。
-- PostgreSQL 不支持 MySQL 的内联 ENUM、ON UPDATE CURRENT_TIMESTAMP、INDEX 内联语法，
-- 因此 ENUM 改为 VARCHAR + CHECK 约束，updated_at 自动更新通过通用触发器函数实现。

-- ============================================================================
-- 通用 updated_at 自动更新触发器函数
-- 所有含 updated_at 列的表在 UPDATE 时自动刷新该列，等价于 MySQL 的
-- ON UPDATE CURRENT_TIMESTAMP(3)。
-- ============================================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP(3);
    RETURN NEW;
END;
$$ language 'plpgsql';

-- ============================================================================
-- 用户表
-- ============================================================================
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL,
    phone VARCHAR(20) NOT NULL,
    password VARCHAR(255) NULL,
    weapp_open_id VARCHAR(100) NULL,
    union_id VARCHAR(100) NULL,
    nickname VARCHAR(50) NOT NULL,
    avatar TEXT NULL,
    bio VARCHAR(500) NULL,
    gender VARCHAR(10) NOT NULL DEFAULT 'UNKNOWN',
    birthday DATE NULL,
    role VARCHAR(10) NOT NULL DEFAULT 'USER',
    status VARCHAR(10) NOT NULL DEFAULT 'ACTIVE',
    streak_days INT NOT NULL DEFAULT 0,
    last_check_in_at TIMESTAMP(3) WITH TIME ZONE NULL,
    following_count INT NOT NULL DEFAULT 0,
    follower_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    CONSTRAINT users_phone_key UNIQUE (phone),
    CONSTRAINT users_weapp_open_id_key UNIQUE (weapp_open_id),
    CONSTRAINT users_union_id_key UNIQUE (union_id),
    CONSTRAINT users_gender_check CHECK (gender IN ('UNKNOWN', 'MALE', 'FEMALE')),
    CONSTRAINT users_role_check CHECK (role IN ('USER', 'ADMIN')),
    CONSTRAINT users_status_check CHECK (status IN ('ACTIVE', 'BANNED')),
    PRIMARY KEY (id)
);

CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- 圈子表
-- ============================================================================
CREATE TABLE IF NOT EXISTS circles (
    id BIGSERIAL,
    name VARCHAR(100) NOT NULL,
    description TEXT NULL,
    cover TEXT NULL,
    theme_color VARCHAR(20) NULL,
    is_public BOOLEAN NOT NULL DEFAULT true,
    member_count INT NOT NULL DEFAULT 0,
    post_count INT NOT NULL DEFAULT 0,
    owner_id BIGINT NOT NULL,
    created_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id)
);

CREATE TRIGGER update_circles_updated_at
    BEFORE UPDATE ON circles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- 圈子成员关联表
-- ============================================================================
CREATE TABLE IF NOT EXISTS circle_members (
    id BIGSERIAL,
    user_id BIGINT NOT NULL,
    circle_id BIGINT NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'MEMBER',
    created_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    CONSTRAINT circle_members_user_id_circle_id_key UNIQUE (user_id, circle_id),
    CONSTRAINT circle_members_role_check CHECK (role IN ('MEMBER', 'MODERATOR', 'OWNER')),
    PRIMARY KEY (id)
);

CREATE TRIGGER update_circle_members_updated_at
    BEFORE UPDATE ON circle_members
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- 话题表
-- 每个新帖子必须关联一个有效话题；旧帖子允许 topic_id 为空。
-- ============================================================================
CREATE TABLE IF NOT EXISTS topics (
    id BIGSERIAL,
    slug VARCHAR(80) NOT NULL,
    title VARCHAR(50) NOT NULL,
    description VARCHAR(500) NULL,
    status INT NOT NULL DEFAULT 1,
    sort INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    CONSTRAINT topics_slug_key UNIQUE (slug),
    CONSTRAINT topics_title_key UNIQUE (title),
    PRIMARY KEY (id)
);

CREATE TRIGGER update_topics_updated_at
    BEFORE UPDATE ON topics
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- 帖子表
-- ============================================================================
CREATE TABLE IF NOT EXISTS posts (
    id BIGSERIAL,
    author_id BIGINT NOT NULL,
    circle_id BIGINT NULL,
    topic_id BIGINT NULL,
    type VARCHAR(20) NOT NULL DEFAULT 'TEXT',
    title VARCHAR(200) NULL,
    content TEXT NOT NULL,
    link_url TEXT NULL,
    like_count INT NOT NULL DEFAULT 0,
    comment_count INT NOT NULL DEFAULT 0,
    share_count INT NOT NULL DEFAULT 0,
    is_pinned BOOLEAN NOT NULL DEFAULT false,
    is_essence BOOLEAN NOT NULL DEFAULT false,
    status INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    CONSTRAINT posts_type_check CHECK (type IN ('TEXT', 'IMAGE', 'VIDEO', 'LINK')),
    PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS posts_topic_id_created_at_idx ON posts (topic_id, created_at);
CREATE INDEX IF NOT EXISTS posts_circle_id_status_author_id_created_at_idx ON posts (circle_id, status, author_id, created_at);

CREATE TRIGGER update_posts_updated_at
    BEFORE UPDATE ON posts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- 帖子媒体资源表
-- ============================================================================
CREATE TABLE IF NOT EXISTS post_medias (
    id BIGSERIAL,
    post_id BIGINT NOT NULL,
    type VARCHAR(20) NOT NULL,
    url TEXT NOT NULL,
    sort INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id)
);

-- ============================================================================
-- 评论表（支持一级评论与二级回复）
-- ============================================================================
CREATE TABLE IF NOT EXISTS comments (
    id BIGSERIAL,
    post_id BIGINT NOT NULL,
    author_id BIGINT NOT NULL,
    parent_id BIGINT NULL,
    reply_to_id BIGINT NULL,
    content TEXT NOT NULL,
    like_count INT NOT NULL DEFAULT 0,
    status INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id)
);

CREATE TRIGGER update_comments_updated_at
    BEFORE UPDATE ON comments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- 点赞表（对帖子或评论点赞）
-- ============================================================================
CREATE TABLE IF NOT EXISTS likes (
    id BIGSERIAL,
    user_id BIGINT NOT NULL,
    target_type VARCHAR(20) NOT NULL,
    target_id BIGINT NOT NULL,
    created_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    CONSTRAINT likes_user_id_target_type_target_id_key UNIQUE (user_id, target_type, target_id),
    PRIMARY KEY (id)
);

-- ============================================================================
-- 私信消息表
-- ============================================================================
CREATE TABLE IF NOT EXISTS messages (
    id BIGSERIAL,
    sender_id BIGINT NOT NULL,
    receiver_id BIGINT NOT NULL,
    type VARCHAR(20) NOT NULL DEFAULT 'TEXT',
    content TEXT NOT NULL,
    is_read BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    CONSTRAINT messages_type_check CHECK (type IN ('TEXT', 'IMAGE')),
    PRIMARY KEY (id)
);

-- ============================================================================
-- 通知表
-- ============================================================================
CREATE TABLE IF NOT EXISTS notifications (
    id BIGSERIAL,
    user_id BIGINT NOT NULL,
    type VARCHAR(20) NOT NULL,
    actor_id BIGINT NULL,
    target_type VARCHAR(20) NULL,
    target_id BIGINT NULL,
    title VARCHAR(200) NOT NULL,
    content TEXT NULL,
    is_read BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    CONSTRAINT notifications_type_check CHECK (type IN ('LIKE', 'COMMENT', 'FOLLOW', 'SYSTEM', 'CIRCLE_INVITE', 'TASK')),
    PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS notifications_user_id_is_read_created_at_idx ON notifications (user_id, is_read, created_at);

-- ============================================================================
-- 关注表
-- ============================================================================
CREATE TABLE IF NOT EXISTS follows (
    id BIGSERIAL,
    follower_id BIGINT NOT NULL,
    following_id BIGINT NOT NULL,
    created_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    CONSTRAINT follows_follower_id_following_id_key UNIQUE (follower_id, following_id),
    PRIMARY KEY (id)
);

-- ============================================================================
-- 打卡记录表
-- post_id 触发本次打卡的帖子；check_in_date 为 Asia/Shanghai 业务日期。
-- ============================================================================
CREATE TABLE IF NOT EXISTS check_ins (
    id BIGSERIAL,
    user_id BIGINT NOT NULL,
    post_id BIGINT NULL,
    check_in_date CHAR(10) NULL,
    circle_id BIGINT NULL,
    content TEXT NULL,
    images TEXT NULL,
    created_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    CONSTRAINT check_ins_post_id_key UNIQUE (post_id),
    CONSTRAINT check_ins_user_id_check_in_date_key UNIQUE (user_id, check_in_date),
    PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS check_ins_user_id_created_at_idx ON check_ins (user_id, created_at);

-- ============================================================================
-- 课程表
-- ============================================================================
CREATE TABLE IF NOT EXISTS courses (
    id BIGSERIAL,
    title VARCHAR(200) NOT NULL,
    description TEXT NULL,
    cover TEXT NULL,
    circle_id BIGINT NULL,
    creator_id BIGINT NOT NULL,
    status INT NOT NULL DEFAULT 1,
    created_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id)
);

CREATE TRIGGER update_courses_updated_at
    BEFORE UPDATE ON courses
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- 课程章节表
-- ============================================================================
CREATE TABLE IF NOT EXISTS course_chapters (
    id BIGSERIAL,
    course_id BIGINT NOT NULL,
    title VARCHAR(200) NOT NULL,
    content TEXT NULL,
    video_url TEXT NULL,
    sort INT NOT NULL DEFAULT 0,
    duration INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id)
);

CREATE TRIGGER update_course_chapters_updated_at
    BEFORE UPDATE ON course_chapters
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- 课程学习进度表
-- ============================================================================
CREATE TABLE IF NOT EXISTS course_progresses (
    id BIGSERIAL,
    user_id BIGINT NOT NULL,
    course_id BIGINT NOT NULL,
    current_chapter_id BIGINT NULL,
    completed_chapter_ids TEXT NULL,
    progress INT NOT NULL DEFAULT 0,
    is_completed BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at TIMESTAMP(3) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    CONSTRAINT course_progresses_user_id_course_id_key UNIQUE (user_id, course_id),
    PRIMARY KEY (id)
);

CREATE TRIGGER update_course_progresses_updated_at
    BEFORE UPDATE ON course_progresses
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- 外键约束
-- ============================================================================
ALTER TABLE circles ADD CONSTRAINT circles_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE circle_members ADD CONSTRAINT circle_members_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE circle_members ADD CONSTRAINT circle_members_circle_id_fkey FOREIGN KEY (circle_id) REFERENCES circles(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE posts ADD CONSTRAINT posts_author_id_fkey FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE posts ADD CONSTRAINT posts_circle_id_fkey FOREIGN KEY (circle_id) REFERENCES circles(id) ON DELETE SET NULL ON UPDATE CASCADE;
ALTER TABLE posts ADD CONSTRAINT posts_topic_id_fkey FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE SET NULL ON UPDATE CASCADE;
ALTER TABLE post_medias ADD CONSTRAINT post_medias_post_id_fkey FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE comments ADD CONSTRAINT comments_post_id_fkey FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE comments ADD CONSTRAINT comments_author_id_fkey FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE comments ADD CONSTRAINT comments_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES comments(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE likes ADD CONSTRAINT likes_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE messages ADD CONSTRAINT messages_sender_id_fkey FOREIGN KEY (sender_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE messages ADD CONSTRAINT messages_receiver_id_fkey FOREIGN KEY (receiver_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE notifications ADD CONSTRAINT notifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE follows ADD CONSTRAINT follows_follower_id_fkey FOREIGN KEY (follower_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE follows ADD CONSTRAINT follows_following_id_fkey FOREIGN KEY (following_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE check_ins ADD CONSTRAINT check_ins_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE check_ins ADD CONSTRAINT check_ins_post_id_fkey FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE SET NULL ON UPDATE CASCADE;
ALTER TABLE courses ADD CONSTRAINT courses_creator_id_fkey FOREIGN KEY (creator_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE courses ADD CONSTRAINT courses_circle_id_fkey FOREIGN KEY (circle_id) REFERENCES circles(id) ON DELETE SET NULL ON UPDATE CASCADE;
ALTER TABLE course_chapters ADD CONSTRAINT course_chapters_course_id_fkey FOREIGN KEY (course_id) REFERENCES courses(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE course_progresses ADD CONSTRAINT course_progresses_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE course_progresses ADD CONSTRAINT course_progresses_course_id_fkey FOREIGN KEY (course_id) REFERENCES courses(id) ON DELETE CASCADE ON UPDATE CASCADE;

-- ============================================================================
-- 种子数据：系统话题
-- 在迁移内写入以保证默认话题在任何应用实例启动前就可用。
-- PostgreSQL 使用 ON CONFLICT 实现幂等 upsert，等价于 MySQL 的 ON DUPLICATE KEY UPDATE。
-- ============================================================================
INSERT INTO topics (slug, title, description, status, sort, created_at, updated_at)
VALUES
    ('weekly-reading', '本周精读', '拆解一本书的关键观点', 1, 30, CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('check-in-challenge', '打卡挑战', '记录每天的阅读与成长', 1, 100, CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('course-resources', '课程资料', '文档、视频、回放统一收纳', 1, 20, CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3))
ON CONFLICT (slug) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    status = EXCLUDED.status,
    sort = EXCLUDED.sort,
    updated_at = CURRENT_TIMESTAMP(3);
