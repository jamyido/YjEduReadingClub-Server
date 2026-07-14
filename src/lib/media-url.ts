/**
 * 图片地址校验结果。
 */
export type ImageUrlValidationResult =
  | { ok: true; url: string }
  | { ok: false; message: string; code: string }

/**
 * 判断图片地址是否仍为浏览器或小程序选择器返回的本地临时路径。
 * 这些地址只在当前客户端会话有效，禁止写入数据库。
 * @param url 原始图片地址
 * @returns 临时地址返回 true
 */
export function isTemporaryMediaUrl(url: string): boolean {
  var normalized = String(url || '').trim().toLowerCase()
  return normalized.indexOf('**tmp**') >= 0
    || normalized.indexOf('/tmp/') >= 0
    || normalized.indexOf('http://tmp') === 0
    || normalized.indexOf('wxfile://') === 0
    || normalized.indexOf('blob:') === 0
    || normalized.indexOf('data:') === 0
    || normalized.indexOf('file:') === 0
    || normalized.indexOf('127.0.0.1') >= 0
    || normalized.indexOf('localhost') >= 0
}

/**
 * 将历史上保存的服务端完整上传 URL 归一化为稳定相对路径。
 * 外部 HTTPS 图片保持原样，只有 pathname 为 /uploads/* 的地址会被收口。
 * @param url 原始图片地址
 * @returns 适合写入数据库的地址
 */
export function normalizePersistedMediaUrl(url: string): string {
  var value = String(url || '').trim()
  if (!value) {
    return ''
  }
  if (value.indexOf('uploads/') === 0 || value.indexOf('assets/') === 0) {
    value = '/' + value
  }
  if (value.indexOf('/uploads/') === 0 || value.indexOf('/assets/') === 0) {
    return value
  }
  if (value.indexOf('http://') !== 0 && value.indexOf('https://') !== 0) {
    return value
  }

  try {
    var parsed = new URL(value)
    if (parsed.pathname.indexOf('/uploads/') === 0) {
      return parsed.pathname
    }
  } catch (error) {
    return value
  }
  return value
}

/**
 * 校验准备持久化的图片地址，并将本服务上传的完整 URL 转为相对路径。
 * 允许本项目 /assets/*、本服务 /uploads/* 与稳定的 HTTP(S) 外链；
 * 新的 /uploads/* 地址必须属于当前用户，已有地址原样回传时保持兼容。
 * @param rawUrl 客户端提交的图片地址
 * @param ownerId 当前上传文件所属用户 ID；注册前等场景可不传
 * @param existingUrl 数据库中已有的图片地址，用于允许未修改的历史值
 * @returns 校验结果与归一化后的稳定地址
 */
export function validatePersistedImageUrl(
  rawUrl: string,
  ownerId?: number,
  existingUrl?: string | null
): ImageUrlValidationResult {
  var trimmed = String(rawUrl || '').trim()
  if (!trimmed) {
    return { ok: true, url: '' }
  }
  if (isTemporaryMediaUrl(trimmed)) {
    return {
      ok: false,
      message: '图片尚未完成上传，请重新选择',
      code: 'TEMP_MEDIA_URL'
    }
  }

  var normalized = normalizePersistedMediaUrl(trimmed)
  if (normalized.indexOf('/assets/') === 0) {
    return { ok: true, url: normalized }
  }
  if (normalized.indexOf('/uploads/') === 0) {
    var normalizedExisting = existingUrl
      ? normalizePersistedMediaUrl(existingUrl)
      : ''
    if (normalizedExisting && normalizedExisting === normalized) {
      return { ok: true, url: normalized }
    }
    if (!/^\/uploads\/\d+\/[A-Za-z0-9._-]+$/.test(normalized)) {
      return {
        ok: false,
        message: '图片上传地址格式无效',
        code: 'INVALID_MEDIA_URL'
      }
    }
    if (!ownerId) {
      return {
        ok: false,
        message: '上传图片缺少所属用户信息',
        code: 'INVALID_MEDIA_OWNER'
      }
    }
    if (normalized.indexOf('/uploads/' + String(ownerId) + '/') !== 0) {
      return {
        ok: false,
        message: '图片路径与当前用户不匹配',
        code: 'INVALID_MEDIA_OWNER'
      }
    }
    return { ok: true, url: normalized }
  }
  if (normalized.indexOf('http://') === 0 || normalized.indexOf('https://') === 0) {
    return { ok: true, url: normalized }
  }

  return {
    ok: false,
    message: '图片地址格式无效',
    code: 'INVALID_MEDIA_URL'
  }
}
