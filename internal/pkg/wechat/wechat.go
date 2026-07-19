// Package wechat 封装微信小程序服务端 API 集成。
// 职责：code2session 换取 openid、获取并缓存 access_token、获取手机号。
package wechat

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"yjedu-reading-club-server/internal/config"
)

// Code2SessionResult 是 code2session 接口返回结构。
type Code2SessionResult struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid,omitempty"`
}

// accessTokenCache 是 access_token 内存缓存，有效期 2 小时，提前 5 分钟刷新。
type accessTokenCache struct {
	token     string
	expiresAt time.Time
}

// wechatError 是微信接口统一错误返回结构。
type wechatError struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// globalAccessTokenCache 与其互斥锁，保证并发安全。
var (
	globalAccessTokenCache *accessTokenCache
	tokenMutex             sync.Mutex
)

// httpClient 是复用的 HTTP 客户端，设置统一超时。
var httpClient = &http.Client{Timeout: 10 * time.Second}

// getAppID 获取微信小程序 AppID，未配置时返回错误。
func getAppID() (string, error) {
	cfg := config.Get()
	if cfg == nil || cfg.WechatMiniAppID == "" {
		return "", errors.New("WECHAT_MINI_APP_ID 环境变量未配置")
	}
	return cfg.WechatMiniAppID, nil
}

// getAppSecret 获取微信小程序 AppSecret，未配置时返回错误。
func getAppSecret() (string, error) {
	cfg := config.Get()
	if cfg == nil || cfg.WechatMiniAppSecret == "" {
		return "", errors.New("WECHAT_MINI_APP_SECRET 环境变量未配置")
	}
	return cfg.WechatMiniAppSecret, nil
}

// parseWechatError 解析微信接口返回的 errcode，非 0 时返回错误。
func parseWechatError(body []byte) error {
	var weErr wechatError
	if err := json.Unmarshal(body, &weErr); err != nil {
		return nil
	}
	if weErr.ErrCode != 0 {
		return errors.New("微信接口错误: " + itoa(weErr.ErrCode) + " " + weErr.ErrMsg)
	}
	return nil
}

// itoa 是轻量整数转字符串实现。
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := false
	if value < 0 {
		negative = true
		value = -value
	}
	buf := [20]byte{}
	pos := len(buf)
	for value > 0 {
		pos--
		buf[pos] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// Code2Session 通过 wx.login 的 code 换取 openid 与 session_key。
// 文档: https://developers.weixin.qq.com/miniprogram/dev/api-backend/open-api/login/auth.code2Session.html
func Code2Session(code string) (*Code2SessionResult, error) {
	appID, err := getAppID()
	if err != nil {
		return nil, err
	}
	secret, err := getAppSecret()
	if err != nil {
		return nil, err
	}
	requestURL := "https://api.weixin.qq.com/sns/jscode2session?appid=" + appID +
		"&secret=" + secret + "&js_code=" + code + "&grant_type=authorization_code"

	resp, err := httpClient.Get(requestURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := parseWechatError(body); err != nil {
		return nil, err
	}
	var result Code2SessionResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetAccessToken 获取微信全局接口调用凭据 access_token。
// 内置内存缓存，提前 5 分钟刷新避免过期。
func GetAccessToken() (string, error) {
	tokenMutex.Lock()
	defer tokenMutex.Unlock()

	now := time.Now()
	buffer := 5 * time.Minute
	// 提前 buffer 时间刷新，避免使用即将过期的 token。
	if globalAccessTokenCache != nil && globalAccessTokenCache.expiresAt.After(now.Add(buffer)) {
		return globalAccessTokenCache.token, nil
	}

	appID, err := getAppID()
	if err != nil {
		return "", err
	}
	secret, err := getAppSecret()
	if err != nil {
		return "", err
	}
	requestURL := "https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=" + appID + "&secret=" + secret

	resp, err := httpClient.Get(requestURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if err := parseWechatError(body); err != nil {
		return "", err
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", err
	}
	globalAccessTokenCache = &accessTokenCache{
		token:     tokenResp.AccessToken,
		expiresAt: now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}
	return tokenResp.AccessToken, nil
}

// GetPhoneNumber 通过 getPhoneNumber 回调 code 换取用户手机号。
// 返回纯手机号字符串（不带国际区号）。
func GetPhoneNumber(phoneCode string) (string, error) {
	accessToken, err := GetAccessToken()
	if err != nil {
		return "", err
	}
	requestURL := "https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token=" + accessToken

	payload, _ := json.Marshal(map[string]string{"code": phoneCode})
	resp, err := httpClient.Post(requestURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if err := parseWechatError(body); err != nil {
		return "", err
	}

	var result struct {
		PhoneInfo struct {
			PhoneNumber string `json:"phoneNumber"`
		} `json:"phone_info"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	return result.PhoneInfo.PhoneNumber, nil
}
