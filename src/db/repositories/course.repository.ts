import { Prisma, Course, CourseProgress } from '@prisma/client'
import prisma from '@/lib/prisma'

/**
 * 课程列表查询选项
 */
export type CourseListOptions = {
  page?: number
  pageSize?: number
  circleId?: number
  creatorId?: number
}

/**
 * 课程仓库
 *
 * 职责：
 * - 封装课程及其章节、学习进度的数据库操作
 * - 提供课程列表、详情、进度更新等接口
 */
export const CourseRepository = {
  /**
   * 根据 ID 查询课程详情（包含章节）
   * @param id 课程 ID
   * @returns 课程详情或 null
   */
  async findById(id: number) {
    return prisma.course.findUnique({
      where: { id },
      include: {
        creator: {
          select: { id: true, nickname: true, avatar: true }
        },
        circle: {
          select: { id: true, name: true }
        },
        chapters: {
          orderBy: { sort: 'asc' }
        }
      }
    })
  },

  /**
   * 分页查询课程列表
   * @param options 查询选项
   * @returns 课程列表与总数
   */
  async findMany(
    options: CourseListOptions = {}
  ): Promise<{ list: Course[]; total: number }> {
    const { page = 1, pageSize = 20, circleId, creatorId } = options
    const skip = (page - 1) * pageSize

    const where: Prisma.CourseWhereInput = { status: 1 }
    if (circleId) where.circleId = circleId
    if (creatorId) where.creatorId = creatorId

    const [list, total] = await Promise.all([
      prisma.course.findMany({
        where,
        skip,
        take: pageSize,
        include: {
          creator: {
            select: { id: true, nickname: true, avatar: true }
          },
          circle: {
            select: { id: true, name: true }
          },
          _count: {
            select: { chapters: true }
          }
        },
        orderBy: { createdAt: 'desc' }
      }),
      prisma.course.count({ where })
    ])

    return { list, total }
  },

  /**
   * 查询用户在某课程的学习进度
   * @param userId 用户 ID
   * @param courseId 课程 ID
   * @returns 学习进度记录或 null
   */
  async findProgress(
    userId: number,
    courseId: number
  ): Promise<CourseProgress | null> {
    return prisma.courseProgress.findUnique({
      where: { userId_courseId: { userId, courseId } }
    })
  },

  /**
   * 创建或更新学习进度
   * @param userId 用户 ID
   * @param courseId 课程 ID
   * @param data 进度更新数据
   * @returns 更新后的进度记录
   */
  async upsertProgress(
    userId: number,
    courseId: number,
    data: {
      currentChapterId?: number
      completedChapterIds?: string
      progress?: number
      isCompleted?: boolean
    }
  ): Promise<CourseProgress> {
    return prisma.courseProgress.upsert({
      where: { userId_courseId: { userId, courseId } },
      create: {
        userId,
        courseId,
        currentChapterId: data.currentChapterId,
        completedChapterIds: data.completedChapterIds,
        progress: data.progress || 0,
        isCompleted: data.isCompleted || false
      },
      update: {
        currentChapterId: data.currentChapterId,
        completedChapterIds: data.completedChapterIds,
        progress: data.progress,
        isCompleted: data.isCompleted
      }
    })
  }
}
