import type { NextApiRequest, NextApiResponse } from 'next'
import fs from 'fs'
import path from 'path'
import { getCurrentUser } from '@/lib/auth-context'
import { sendSuccess, sendError, sendUnauthorized } from '@/lib/api-response'
import type { ApiResponse } from '@/lib/api-response'

/**
 * 上传响应数据
 */
type UploadData = {
  url: string
  userId: number
  kind: 'avatar' | 'circle' | 'post'
}

/**
 * 文件上传接口
 *
 * 用于头像、圈子封面与帖子图片上传。文件按用户 ID 保存到
 * server/public/uploads/{userId}/ 目录，避免不同用户文件混放。
 *
 * POST /api/upload
 * FormData: { file: File }
 * @returns 上传后的文件 URL
 */
export const config = {
  api: {
    bodyParser: false
  }
}

/**
 * 从请求中读取 multipart/form-data 数据并提取文件
 * @param req NextApiRequest
 * @returns 文件 Buffer、文件名、MIME 类型
 */
function parseMultipartFormData(req: NextApiRequest): Promise<{
  buffer: Buffer
  filename: string
  mimetype: string
}> {
  return new Promise(function (resolve, reject) {
    var chunks: Buffer[] = []
    var contentType = req.headers['content-type'] || ''
    var boundaryMatch = contentType.match(/boundary=(.+)$/)

    if (!boundaryMatch) {
      reject(new Error('无效的 Content-Type'))
      return
    }

    var boundary = boundaryMatch[1].trim().replace(/^"|"$/g, '')

    req.on('data', function (chunk) {
      chunks.push(chunk as Buffer)
    })

    req.on('end', function () {
      var boundaryMarker = '--' + boundary
      var body = Buffer.concat(chunks)
      var boundaryBuffer = Buffer.from(boundaryMarker)

      // 查找文件部分
      var partStart = body.indexOf(boundaryBuffer)
      if (partStart === -1) {
        reject(new Error('未找到 boundary'))
        return
      }

      // 查找文件头
      var headerStart = body.indexOf('Content-Disposition:', partStart)
      var headerEnd = body.indexOf('\r\n\r\n', headerStart)
      if (headerEnd === -1) {
        reject(new Error('未找到文件头结束位置'))
        return
      }

      var headerStr = body.slice(headerStart, headerEnd).toString()
      var filenameMatch = headerStr.match(/filename="(.+?)"/)
      if (!filenameMatch) {
        reject(new Error('未找到文件名'))
        return
      }

      var filename = filenameMatch[1]
      var mimetypeMatch = headerStr.match(/Content-Type: (.+)/)
      var mimetype = mimetypeMatch ? mimetypeMatch[1].trim() : 'application/octet-stream'

      // 文件内容
      var fileStart = headerEnd + 4
      var nextBoundary = Buffer.from('\r\n' + boundaryMarker)
      var fileEnd = body.indexOf(nextBoundary, fileStart)
      if (fileEnd === -1) {
        reject(new Error('未找到文件内容结束位置'))
        return
      }

      var fileBuffer = body.slice(fileStart, fileEnd)
      resolve({ buffer: fileBuffer, filename: filename, mimetype: mimetype })
    })

    req.on('error', function (err) {
      reject(err)
    })
  })
}

/**
 * 推导后端服务根目录（即 server/ 目录）的绝对路径。
 *
 * 上传接口原本使用 process.cwd() 拼接 public/uploads，但 process.cwd()
 * 取决于启动 Next.js 进程时的工作目录。若部署时未从 server/ 目录启动
 * （例如从项目根目录或通过 PM2/Docker 指定其他 cwd），写入路径会偏离
 * server/public/uploads，而 Next.js 静态文件服务始终从 server/public/
 * 读取，导致访问 /uploads/* 返回 404。
 *
 * 这里改为从 __dirname 向上查找同时包含 public/ 与 package.json 的
 * 祖先目录，确保 dev 与 production build 两种模式下都能定位到 server/。
 * @returns server/ 目录绝对路径
 */
function getServerRoot(): string {
  var current = __dirname
  var i = 0
  while (current && i < 10) {
    if (fs.existsSync(path.join(current, 'public')) &&
        fs.existsSync(path.join(current, 'package.json'))) {
      return current
    }
    var parent = path.dirname(current)
    if (parent === current) {
      break
    }
    current = parent
    i++
  }
  // 兜底回退到 process.cwd()，保留原有行为
  return process.cwd()
}

export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ApiResponse<UploadData>>
) {
  if (req.method !== 'POST') {
    return sendError(res, '仅支持 POST 请求', 'METHOD_NOT_ALLOWED', 405)
  }

  // 校验登录态
  const user = await getCurrentUser(req)
  if (!user) {
    return sendUnauthorized(res, '请先登录')
  }

  try {
    var parsed = await parseMultipartFormData(req)
    var { buffer, mimetype } = parsed
    var rawKind = Array.isArray(req.query.kind) ? req.query.kind[0] : req.query.kind
    if (rawKind && rawKind !== 'avatar' && rawKind !== 'circle' && rawKind !== 'post') {
      return sendError(res, '上传图片用途无效', 'INVALID_UPLOAD_KIND')
    }
    var kind: 'avatar' | 'circle' | 'post' = rawKind === 'circle'
      ? 'circle'
      : rawKind === 'post'
        ? 'post'
        : 'avatar'
    var normalizedMimetype = mimetype.toLowerCase()

    // 仅允许图片类型
    if (normalizedMimetype.indexOf('image/') !== 0) {
      return sendError(res, '仅支持图片上传', 'INVALID_FILE_TYPE')
    }

    if (buffer.length === 0) {
      return sendError(res, '上传文件不能为空', 'EMPTY_FILE')
    }

    // 限制文件大小 10MB
    if (buffer.length > 10 * 1024 * 1024) {
      return sendError(res, '文件大小不能超过 10MB', 'FILE_TOO_LARGE')
    }

    // 根据 MIME 类型生成可信扩展名，避免直接使用客户端文件名造成路径或格式风险。
    var extensionMap: Record<string, string> = {
      'image/jpeg': '.jpg',
      'image/jpg': '.jpg',
      'image/png': '.png',
      'image/webp': '.webp',
      'image/gif': '.gif'
    }
    var ext = extensionMap[normalizedMimetype]
    if (!ext) {
      return sendError(res, '仅支持 JPG、PNG、WebP 或 GIF 图片', 'UNSUPPORTED_IMAGE_TYPE')
    }
    var uniqueName = kind + '_' + Date.now() + '_' + Math.random().toString(36).substring(2, 10) + ext

    // 每个用户使用独立目录：uploads/{userId}/
    // 上传目录放在 server/uploads/（即项目根目录下的 uploads/，不在 public/ 下），
    // 由 pages/api/uploads/[...path].ts 路由实时读取返回（对外 /uploads/* 路径
    // 经 next.config.js rewrite 到 /api/uploads/*），避免 Next.js dev 模式下
    // public/ 静态文件缓存导致 400。
    var userDirectory = String(user.id)
    var uploadDir = path.join(getServerRoot(), 'uploads', userDirectory)
    if (!fs.existsSync(uploadDir)) {
      fs.mkdirSync(uploadDir, { recursive: true })
    }

    var filePath = path.join(uploadDir, uniqueName)
    fs.writeFileSync(filePath, buffer)

    var url = '/uploads/' + userDirectory + '/' + uniqueName

    return sendSuccess(res, { url, userId: user.id, kind }, '上传成功')
  } catch (error) {
    var message = error instanceof Error ? error.message : '上传失败'
    return sendError(res, message, 'UPLOAD_FAILED', 500)
  }
}
