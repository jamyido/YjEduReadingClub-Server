/**
 * 微信小程序服务端 API 集成
 *
 * 职责：
 * - 通过 wx.login 的 code 换取 openid 与 session_key（code2session）
 * - 获取并缓存全局接口调用凭据 access_token
 * - 通过 getPhoneNumber 的 code 换取用户手机号
 */

/**
 * code2session 接口返回结构
 */
interface Code2SessionResult {
  openid: string
  session_key: string
  unionid?: string
}

/**
 * access_token 缓存结构
 */
interface AccessTokenCache {
  token: string
  expiresAt: number
}

/**
 * 全局 access_token 内存缓存
 * access_token 有效期 2 小时，提前 5 分钟刷新
 */
let accessTokenCache: AccessTokenCache | null = null

/**
 * 获取微信小程序 AppID
 * @returns AppID 字符串
 */
function getAppId(): string {
  const appId = process.env.WECHAT_MINI_APP_ID
  if (!appId) {
    throw new Error('WECHAT_MINI_APP_ID 环境变量未配置')
  }
  return appId
}

/**
 * 获取微信小程序 AppSecret
 * @returns AppSecret 字符串
 */
function getAppSecret(): string {
  const secret = process.env.WECHAT_MINI_APP_SECRET
  if (!secret) {
    throw new Error('WECHAT_MINI_APP_SECRET 环境变量未配置')
  }
  return secret
}

/**
 * 通过 wx.login 的 code 换取 openid 与 session_key
 * 文档: https://developers.weixin.qq.com/miniprogram/dev/api-backend/open-api/login/auth.code2Session.html
 * @param code 小程序端 wx.login() 返回的 code
 * @returns openid、session_key 及可选的 unionid
 */
export async function code2Session(code: string): Promise<Code2SessionResult> {
  const appId = getAppId()
  const secret = getAppSecret()
  const url = `https://api.weixin.qq.com/sns/jscode2session?appid=${appId}&secret=${secret}&js_code=${code}&grant_type=authorization_code`

  const res = await fetch(url)
  const data = await res.json()

  if (data.errcode) {
    throw new Error(`微信 code2session 失败: ${data.errcode} ${data.errmsg}`)
  }

  return {
    openid: data.openid,
    session_key: data.session_key,
    unionid: data.unionid
  }
}

/**
 * 获取微信全局接口调用凭据 access_token
 * 内置内存缓存，提前 5 分钟刷新避免过期
 * 文档: https://developers.weixin.qq.com/miniprogram/dev/api-backend/open-api/access-token/auth.getAccessToken.html
 * @returns access_token 字符串
 */
export async function getAccessToken(): Promise<string> {
  const now = Date.now()
  const BUFFER_MS = 5 * 60 * 1000

  if (accessTokenCache && accessTokenCache.expiresAt - BUFFER_MS > now) {
    return accessTokenCache.token
  }

  const appId = getAppId()
  const secret = getAppSecret()
  const url = `https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=${appId}&secret=${secret}`

  const res = await fetch(url)
  const data = await res.json()

  if (data.errcode) {
    throw new Error(`获取 access_token 失败: ${data.errcode} ${data.errmsg}`)
  }

  accessTokenCache = {
    token: data.access_token,
    expiresAt: now + data.expires_in * 1000
  }

  return data.access_token
}

/**
 * 通过 getPhoneNumber 回调 code 换取用户手机号
 * 文档: https://developers.weixin.qq.com/miniprogram/dev/api-backend/open-api/phonenumber/phonenumber.getPhoneNumber.html
 * @param phoneCode 小程序端 button[open-type=getPhoneNumber] 回调中的 code
 * @returns 纯手机号字符串（不带国际区号）
 */
export async function getPhoneNumber(phoneCode: string): Promise<string> {
  const accessToken = await getAccessToken()
  const url = `https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token=${accessToken}`

  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code: phoneCode })
  })
  const data = await res.json()

  if (data.errcode) {
    throw new Error(`获取手机号失败: ${data.errcode} ${data.errmsg}`)
  }

  return data.phone_info.phoneNumber
}
