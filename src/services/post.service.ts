import { Prisma } from '@prisma/client'
import prisma from '@/lib/prisma'
import { CheckInRepository, PostRepository } from '@/db/repositories'
import {
  CHECK_IN_TOPIC_SLUG,
  getEffectiveStreakDays,
  getPreviousShanghaiDateKey,
  toShanghaiDateKey
} from '@/lib/business-date'

/** 并发冲突时事务最大尝试次数。 */
const MAX_TRANSACTION_ATTEMPTS = 3

/** 帖子媒体输入。 */
export type PostServiceMediaInput = {
  type: string
  url: string
  sort?: number
}

/** 发帖事务输入。 */
export type CreatePostServiceInput = {
  authorId: number
  circleId?: number
  topicId?: number
  type?: string
  title?: string
  content: string
  linkUrl?: string
  medias?: PostServiceMediaInput[]
  // 兼容旧打卡接口：若当天已有打卡，则整个事务返回 409 且不创建帖子。
  requireNewCheckIn?: boolean
}

/** 发帖事务领域错误代码。 */
export type PostServiceErrorCode =
  | 'TOPIC_NOT_FOUND'
  | 'DEFAULT_TOPIC_MISSING'
  | 'CIRCLE_NOT_FOUND'
  | 'NOT_CIRCLE_MEMBER'
  | 'CHECK_IN_TOPIC_REQUIRED'
  | 'ALREADY_CHECKED_IN'

/**
 * 发帖事务的可预期领域错误。
 * API 层可依据 code/statusCode 返回稳定错误，而不是转换为 500。
 */
export class PostServiceError extends Error {
  code: PostServiceErrorCode
  statusCode: number

  constructor(code: PostServiceErrorCode, message: string, statusCode: number) {
    super(message)
    Object.setPrototypeOf(this, PostServiceError.prototype)
    this.name = 'PostServiceError'
    this.code = code
    this.statusCode = statusCode
  }
}

/**
 * 判断 Prisma 错误是否适合重试。
 * P2002 用于同日打卡唯一键竞争，P2034 用于事务写冲突或死锁。
 * @param error 捕获到的异常
 * @returns 是否应重新执行整个事务
 */
function isRetryableTransactionError(error: unknown): boolean {
  if (!(error instanceof Prisma.PrismaClientKnownRequestError)) {
    return false
  }
  return error.code === 'P2002' || error.code === 'P2034'
}

/**
 * 将帖子图片整理为 CheckIn.images 使用的 JSON 字符串。
 * @param medias 帖子媒体列表
 * @returns 图片 URL JSON；没有图片时返回 undefined
 */
function serializeCheckInImages(
  medias: PostServiceMediaInput[] | undefined
): string | undefined {
  if (!medias || medias.length === 0) {
    return undefined
  }

  const imageUrls = medias
    .filter(function (media) { return media.type !== 'video' })
    .map(function (media) { return media.url })

  return imageUrls.length > 0 ? JSON.stringify(imageUrls) : undefined
}

/**
 * 在单个 Prisma 事务中创建帖子并完成所有计数与打卡副作用。
 * @param input 发帖输入
 * @returns 帖子以及可选的当天首次打卡结果
 */
async function runCreateTransaction(input: CreatePostServiceInput) {
  return prisma.$transaction(async function (tx) {
    const now = new Date()

    const topic = input.topicId
      ? await tx.topic.findFirst({
          where: { id: input.topicId, status: 1 }
        })
      : await tx.topic.findUnique({
          where: { slug: CHECK_IN_TOPIC_SLUG }
        })

    if (!topic || topic.status !== 1) {
      if (input.topicId) {
        throw new PostServiceError('TOPIC_NOT_FOUND', '话题不存在或已停用', 400)
      }
      throw new PostServiceError(
        'DEFAULT_TOPIC_MISSING',
        '系统默认打卡话题尚未配置',
        500
      )
    }

    if (input.requireNewCheckIn && topic.slug !== CHECK_IN_TOPIC_SLUG) {
      throw new PostServiceError(
        'CHECK_IN_TOPIC_REQUIRED',
        '打卡必须使用打卡挑战话题',
        400
      )
    }

    if (input.circleId) {
      const circle = await tx.circle.findUnique({
        where: { id: input.circleId },
        select: { id: true }
      })
      if (!circle) {
        throw new PostServiceError('CIRCLE_NOT_FOUND', '圈子不存在', 404)
      }

      const membership = await tx.circleMember.findUnique({
        where: {
          userId_circleId: {
            userId: input.authorId,
            circleId: input.circleId
          }
        },
        select: { id: true }
      })
      if (!membership) {
        throw new PostServiceError(
          'NOT_CIRCLE_MEMBER',
          '加入圈子后才能在该圈子发帖',
          403
        )
      }
    }

    const businessDate = toShanghaiDateKey(now)
    const previousBusinessDate = getPreviousShanghaiDateKey(now)

    // 旧打卡接口要求“当天已有打卡时不创建帖子”，因此必须在创建帖子前校验。
    if (input.requireNewCheckIn) {
      const existingCheckIn = await tx.checkIn.findUnique({
        where: {
          userId_checkInDate: {
            userId: input.authorId,
            checkInDate: businessDate
          }
        }
      })
      const currentUser = await tx.user.findUnique({
        where: { id: input.authorId },
        select: { lastCheckInAt: true }
      })
      const lastDate = currentUser && currentUser.lastCheckInAt
        ? toShanghaiDateKey(currentUser.lastCheckInAt)
        : null

      if (existingCheckIn || lastDate === businessDate) {
        throw new PostServiceError(
          'ALREADY_CHECKED_IN',
          '今日已打卡，请明天再来',
          409
        )
      }
    }

    const post = await PostRepository.create({
      authorId: input.authorId,
      circleId: input.circleId,
      topicId: topic.id,
      type: input.type,
      title: input.title,
      content: input.content,
      linkUrl: input.linkUrl,
      medias: input.medias
    }, tx)

    if (input.circleId) {
      await tx.circle.update({
        where: { id: input.circleId },
        data: { postCount: { increment: 1 } }
      })
    }

    let checkIn = null
    let streakDays = 0

    if (topic.slug === CHECK_IN_TOPIC_SLUG) {
      const currentUser = await tx.user.findUnique({
        where: { id: input.authorId },
        select: { streakDays: true, lastCheckInAt: true }
      })
      const existingCheckIn = await tx.checkIn.findUnique({
        where: {
          userId_checkInDate: {
            userId: input.authorId,
            checkInDate: businessDate
          }
        }
      })
      const lastDate = currentUser && currentUser.lastCheckInAt
        ? toShanghaiDateKey(currentUser.lastCheckInAt)
        : null

      // lastCheckInAt 的同日判断兼容迁移当天尚无 checkInDate 的旧打卡记录。
      if (!existingCheckIn && lastDate !== businessDate) {
        streakDays = lastDate === previousBusinessDate && currentUser
          ? currentUser.streakDays + 1
          : 1

        checkIn = await CheckInRepository.create({
          userId: input.authorId,
          postId: post.id,
          checkInDate: businessDate,
          circleId: input.circleId,
          content: input.content,
          images: serializeCheckInImages(input.medias)
        }, tx)

        await tx.user.update({
          where: { id: input.authorId },
          data: {
            streakDays,
            lastCheckInAt: checkIn.createdAt
          }
        })
      }
    }

    return {
      post,
      checkIn,
      streakDays
    }
  }, {
    isolationLevel: Prisma.TransactionIsolationLevel.Serializable,
    maxWait: 5000,
    timeout: 10000
  })
}

/**
 * 创建帖子；遇到同日打卡唯一键竞争或事务死锁时自动重试整个事务。
 * @param input 发帖输入
 * @returns 完整发帖事务结果
 */
async function create(input: CreatePostServiceInput) {
  let attempt = 0
  while (attempt < MAX_TRANSACTION_ATTEMPTS) {
    attempt += 1
    try {
      return await runCreateTransaction(input)
    } catch (error) {
      if (!isRetryableTransactionError(error) || attempt >= MAX_TRANSACTION_ATTEMPTS) {
        throw error
      }
    }
  }

  throw new Error('创建帖子事务重试失败')
}

/** 发帖领域服务。 */
export const PostService = { create }
