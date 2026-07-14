import type { NextApiResponse } from 'next'

/**
 * 统一 API 响应结构
 */
export interface ApiResponse<T = unknown> {
  success: boolean
  data?: T
  message?: string
  error?: {
    code: string
    message: string
  }
}

/**
 * 分页响应数据结构
 */
export interface PaginatedData<T> {
  list: T[]
  total: number
  page: number
  pageSize: number
  hasMore: boolean
}

/**
 * 发送成功响应
 * @param res Next.js 响应对象
 * @param data 响应数据
 * @param message 可选的成功消息
 * @param statusCode HTTP 状态码，默认 200
 */
export function sendSuccess<T>(
  res: NextApiResponse<ApiResponse<T>>,
  data: T,
  message?: string,
  statusCode: number = 200
): void {
  res.status(statusCode).json({
    success: true,
    data,
    message
  })
}

/**
 * 发送分页列表响应
 * @param res Next.js 响应对象
 * @param list 列表数据
 * @param total 总数
 * @param page 当前页码
 * @param pageSize 每页数量
 */
export function sendPaginated<T>(
  res: NextApiResponse<ApiResponse<PaginatedData<T>>>,
  list: T[],
  total: number,
  page: number,
  pageSize: number
): void {
  const hasMore = page * pageSize < total
  res.status(200).json({
    success: true,
    data: { list, total, page, pageSize, hasMore }
  })
}

/**
 * 发送错误响应
 * 使用泛型 T 使函数能接受任意类型的 ApiResponse 响应对象
 * @param res Next.js 响应对象
 * @param message 错误消息
 * @param code 错误代码
 * @param statusCode HTTP 状态码，默认 400
 */
export function sendError<T = unknown>(
  res: NextApiResponse<ApiResponse<T>>,
  message: string,
  code: string = 'BAD_REQUEST',
  statusCode: number = 400
): void {
  res.status(statusCode).json({
    success: false,
    error: { code, message }
  })
}

/**
 * 发送 401 未认证响应
 * @param res Next.js 响应对象
 * @param message 可选的自定义消息
 */
export function sendUnauthorized<T = unknown>(
  res: NextApiResponse<ApiResponse<T>>,
  message: string = '未登录或登录已过期'
): void {
  sendError(res, message, 'UNAUTHORIZED', 401)
}

/**
 * 发送 403 无权限响应
 * @param res Next.js 响应对象
 * @param message 可选的自定义消息
 */
export function sendForbidden<T = unknown>(
  res: NextApiResponse<ApiResponse<T>>,
  message: string = '无权限执行此操作'
): void {
  sendError(res, message, 'FORBIDDEN', 403)
}

/**
 * 发送 404 未找到响应
 * @param res Next.js 响应对象
 * @param message 可选的自定义消息
 */
export function sendNotFound<T = unknown>(
  res: NextApiResponse<ApiResponse<T>>,
  message: string = '资源不存在'
): void {
  sendError(res, message, 'NOT_FOUND', 404)
}

/**
 * 发送 500 服务器内部错误响应
 * @param res Next.js 响应对象
 * @param message 可选的自定义消息
 */
export function sendInternalError<T = unknown>(
  res: NextApiResponse<ApiResponse<T>>,
  message: string = '服务器内部错误'
): void {
  sendError(res, message, 'INTERNAL_ERROR', 500)
}
