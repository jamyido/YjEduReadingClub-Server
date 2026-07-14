/**
 * 数据仓库统一导出入口
 *
 * 职责：
 * - 集中导出所有 Repository，方便服务层统一引入
 * - 减少跨模块导入路径冗余
 */
export { UserRepository } from './user.repository'
export { CircleRepository } from './circle.repository'
export { PostRepository } from './post.repository'
export { CommentRepository } from './comment.repository'
export { FollowRepository } from './follow.repository'
export { MessageRepository } from './message.repository'
export { NotificationRepository } from './notification.repository'
export { CheckInRepository } from './checkin.repository'
export { CourseRepository } from './course.repository'
export { TopicRepository } from './topic.repository'

export type { CreateUserInput, UpdateUserInput } from './user.repository'
export type { CreateCircleInput, UpdateCircleInput, CircleListOptions } from './circle.repository'
export type {
  CreatePostInput,
  UpdatePostInput,
  PostListOptions,
  CirclePostDaysRankingOptions,
  CirclePostDaysRankingRecord
} from './post.repository'
export type { MessageListOptions } from './message.repository'
export type { CheckInListOptions, CreateCheckInInput } from './checkin.repository'
export type { CourseListOptions } from './course.repository'
export type {
  CreateNotificationInput,
  CreateManyNotificationsInput,
  NotificationUnreadTypeCount,
  NotificationListCategory,
  NotificationListOptions
} from './notification.repository'
export type { TopicListOptions } from './topic.repository'
