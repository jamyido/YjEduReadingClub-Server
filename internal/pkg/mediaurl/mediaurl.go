// Package mediaurl 封装用户上传图片地址的校验与归一化逻辑。
// 防止客户端写入临时路径或引用他人目录下的图片。
package mediaurl

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// ImageUrlValidationResult 是图片地址校验结果。
type ImageUrlValidationResult struct {
	OK      bool
	URL     string
	Message string
	Code    string
}

// uploadsLegacyPathRegex 校验旧版 /uploads/{userId}/{filename} 的路径格式。
// 历史数据中已存在该格式，需保持兼容。
var uploadsLegacyPathRegex = regexp.MustCompile(`^/uploads/\d+/[A-Za-z0-9._-]+$`)

// uploadsAvatarPathRegex 校验 /uploads/users/{userId}/avatar/{filename} 的路径格式。
// 新版头像上传按用户ID下的 avatar 子目录归档，便于服务器整理维护。
var uploadsAvatarPathRegex = regexp.MustCompile(`^/uploads/users/\d+/avatar/[A-Za-z0-9._-]+$`)

// uploadsCircleCoverPathRegex 校验 /uploads/circles/circle_{circleId}/{filename} 的路径格式。
// 圈子封面按圈子 ID 分目录归档，加 circle_ 前缀避免与用户 ID 冲突。
var uploadsCircleCoverPathRegex = regexp.MustCompile(`^/uploads/circles/circle_\d+/[A-Za-z0-9._-]+$`)

// uploadsCircleTempPathRegex 校验 /uploads/users/{userId}/circle_temp/{filename} 的路径格式。
// 创建圈子时还没有 circleId，临时存到用户目录下的 circle_temp 子目录。
var uploadsCircleTempPathRegex = regexp.MustCompile(`^/uploads/users/\d+/circle_temp/[A-Za-z0-9._-]+$`)

// matchUploadsPath 判断地址是否符合任一已知的上传路径格式。
func matchUploadsPath(normalized string) bool {
	return uploadsLegacyPathRegex.MatchString(normalized) || uploadsAvatarPathRegex.MatchString(normalized)
}

// matchCircleCoverUploadsPath 判断地址是否符合圈子封面上传路径格式（正式或临时）。
func matchCircleCoverUploadsPath(normalized string) bool {
	return uploadsCircleCoverPathRegex.MatchString(normalized) || uploadsCircleTempPathRegex.MatchString(normalized)
}

// IsTemporaryMediaUrl 判断图片地址是否仍为客户端临时路径。
// 临时地址只在当前客户端会话有效，禁止写入数据库。
func IsTemporaryMediaUrl(rawURL string) bool {
	normalized := strings.ToLower(strings.TrimSpace(rawURL))
	return strings.Contains(normalized, "**tmp**") ||
		strings.Contains(normalized, "/tmp/") ||
		strings.HasPrefix(normalized, "http://tmp") ||
		strings.HasPrefix(normalized, "wxfile://") ||
		strings.HasPrefix(normalized, "blob:") ||
		strings.HasPrefix(normalized, "data:") ||
		strings.HasPrefix(normalized, "file:") ||
		strings.Contains(normalized, "127.0.0.1") ||
		strings.Contains(normalized, "localhost")
}

// NormalizePersistedMediaUrl 将服务端完整上传 URL 归一化为稳定相对路径。
// 外部 HTTPS 图片保持原样，只有 pathname 为 /uploads/* 的地址会被收口。
func NormalizePersistedMediaUrl(rawURL string) string {
	value := strings.TrimSpace(rawURL)
	if value == "" {
		return ""
	}
	// 历史上可能存了不带前导斜杠的相对路径。
	if strings.HasPrefix(value, "uploads/") || strings.HasPrefix(value, "assets/") {
		value = "/" + value
	}
	if strings.HasPrefix(value, "/uploads/") || strings.HasPrefix(value, "/assets/") {
		return value
	}
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		return value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return value
	}
	if strings.HasPrefix(parsed.Path, "/uploads/") {
		return parsed.Path
	}
	return value
}

// ValidatePersistedImageUrl 校验准备持久化的图片地址，并将本服务上传的完整 URL 转为相对路径。
// 允许本项目 /assets/*、本服务 /uploads/* 与稳定的 HTTP(S) 外链；
// 新的 /uploads/* 地址必须属于当前用户，已有地址原样回传时保持兼容。
func ValidatePersistedImageUrl(rawURL string, ownerID int64, existingURL string) ImageUrlValidationResult {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ImageUrlValidationResult{OK: true, URL: ""}
	}
	if IsTemporaryMediaUrl(trimmed) {
		return ImageUrlValidationResult{
			OK:      false,
			Message: "图片尚未完成上传，请重新选择",
			Code:    "TEMP_MEDIA_URL",
		}
	}

	normalized := NormalizePersistedMediaUrl(trimmed)

	// 项目静态资源目录直接放行。
	if strings.HasPrefix(normalized, "/assets/") {
		return ImageUrlValidationResult{OK: true, URL: normalized}
	}

	// 上传目录需校验归属。
	if strings.HasPrefix(normalized, "/uploads/") {
		normalizedExisting := ""
		if existingURL != "" {
			normalizedExisting = NormalizePersistedMediaUrl(existingURL)
		}
		if normalizedExisting != "" && normalizedExisting == normalized {
			return ImageUrlValidationResult{OK: true, URL: normalized}
		}
		if !matchUploadsPath(normalized) {
			return ImageUrlValidationResult{
				OK:      false,
				Message: "图片上传地址格式无效",
				Code:    "INVALID_MEDIA_URL",
			}
		}
		if ownerID == 0 {
			return ImageUrlValidationResult{
				OK:      false,
				Message: "上传图片缺少所属用户信息",
				Code:    "INVALID_MEDIA_OWNER",
			}
		}
		// 同时支持新旧两种归属前缀：
		// 旧版 /uploads/{userId}/ 与新版 /uploads/users/{userId}/avatar/。
		ownerIDStr := strconv.FormatInt(ownerID, 10)
		legacyPrefix := "/uploads/" + ownerIDStr + "/"
		avatarPrefix := "/uploads/users/" + ownerIDStr + "/avatar/"
		if !strings.HasPrefix(normalized, legacyPrefix) && !strings.HasPrefix(normalized, avatarPrefix) {
			return ImageUrlValidationResult{
				OK:      false,
				Message: "图片路径与当前用户不匹配",
				Code:    "INVALID_MEDIA_OWNER",
			}
		}
		return ImageUrlValidationResult{OK: true, URL: normalized}
	}

	// 外部 HTTP(S) 链接直接放行。
	if strings.HasPrefix(normalized, "http://") || strings.HasPrefix(normalized, "https://") {
		return ImageUrlValidationResult{OK: true, URL: normalized}
	}

	return ImageUrlValidationResult{
		OK:      false,
		Message: "图片地址格式无效",
		Code:    "INVALID_MEDIA_URL",
	}
}

// ValidatePersistedImageUrlErr 是 ValidatePersistedImageUrl 的便捷封装，
// 校验失败时返回错误，成功时返回归一化后的地址。
func ValidatePersistedImageUrlErr(rawURL string, ownerID int64, existingURL string) (string, error) {
	result := ValidatePersistedImageUrl(rawURL, ownerID, existingURL)
	if !result.OK {
		return "", errors.New(result.Message)
	}
	return result.URL, nil
}

// ValidatePersistedCircleCoverUrl 校验准备持久化的圈子封面地址。
// 允许本项目 /assets/*、本服务 /uploads/circles/circle_{circleId}/* 与 /uploads/users/{userId}/circle_temp/*（创建时临时）、稳定 HTTP(S) 外链。
// 正式路径必须归属于当前 circleID；临时路径必须归属于当前 userID（创建圈子场景）。
// 已有地址原样回传时保持兼容。
func ValidatePersistedCircleCoverUrl(rawURL string, circleID int64, userID int64, existingURL string) ImageUrlValidationResult {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ImageUrlValidationResult{OK: true, URL: ""}
	}
	if IsTemporaryMediaUrl(trimmed) {
		return ImageUrlValidationResult{
			OK:      false,
			Message: "图片尚未完成上传，请重新选择",
			Code:    "TEMP_MEDIA_URL",
		}
	}

	normalized := NormalizePersistedMediaUrl(trimmed)

	// 项目静态资源目录直接放行。
	if strings.HasPrefix(normalized, "/assets/") {
		return ImageUrlValidationResult{OK: true, URL: normalized}
	}

	// 上传目录需校验归属。
	if strings.HasPrefix(normalized, "/uploads/") {
		normalizedExisting := ""
		if existingURL != "" {
			normalizedExisting = NormalizePersistedMediaUrl(existingURL)
		}
		if normalizedExisting != "" && normalizedExisting == normalized {
			return ImageUrlValidationResult{OK: true, URL: normalized}
		}
		if !matchCircleCoverUploadsPath(normalized) {
			return ImageUrlValidationResult{
				OK:      false,
				Message: "圈子封面地址格式无效",
				Code:    "INVALID_MEDIA_URL",
			}
		}
		// 正式路径 /uploads/circles/circle_{circleId}/ 必须归属于当前 circleID。
		if uploadsCircleCoverPathRegex.MatchString(normalized) {
			if circleID <= 0 {
				return ImageUrlValidationResult{
					OK:      false,
					Message: "圈子封面缺少所属圈子信息",
					Code:    "INVALID_MEDIA_OWNER",
				}
			}
			expectedPrefix := "/uploads/circles/circle_" + strconv.FormatInt(circleID, 10) + "/"
			if !strings.HasPrefix(normalized, expectedPrefix) {
				return ImageUrlValidationResult{
					OK:      false,
					Message: "圈子封面路径与当前圈子不匹配",
					Code:    "INVALID_MEDIA_OWNER",
				}
			}
			return ImageUrlValidationResult{OK: true, URL: normalized}
		}
		// 临时路径 /uploads/users/{userId}/circle_temp/ 必须归属于当前 userID（创建圈子场景）。
		if uploadsCircleTempPathRegex.MatchString(normalized) {
			if userID <= 0 {
				return ImageUrlValidationResult{
					OK:      false,
					Message: "圈子封面缺少所属用户信息",
					Code:    "INVALID_MEDIA_OWNER",
				}
			}
			expectedPrefix := "/uploads/users/" + strconv.FormatInt(userID, 10) + "/circle_temp/"
			if !strings.HasPrefix(normalized, expectedPrefix) {
				return ImageUrlValidationResult{
					OK:      false,
					Message: "圈子封面临时路径与当前用户不匹配",
					Code:    "INVALID_MEDIA_OWNER",
				}
			}
			return ImageUrlValidationResult{OK: true, URL: normalized}
		}
		return ImageUrlValidationResult{OK: true, URL: normalized}
	}

	// 外部 HTTP(S) 链接直接放行。
	if strings.HasPrefix(normalized, "http://") || strings.HasPrefix(normalized, "https://") {
		return ImageUrlValidationResult{OK: true, URL: normalized}
	}

	return ImageUrlValidationResult{
		OK:      false,
		Message: "图片地址格式无效",
		Code:    "INVALID_MEDIA_URL",
	}
}

// ValidatePersistedCircleCoverUrlErr 是 ValidatePersistedCircleCoverUrl 的便捷封装，
// 校验失败时返回错误，成功时返回归一化后的地址。
func ValidatePersistedCircleCoverUrlErr(rawURL string, circleID int64, userID int64, existingURL string) (string, error) {
	result := ValidatePersistedCircleCoverUrl(rawURL, circleID, userID, existingURL)
	if !result.OK {
		return "", errors.New(result.Message)
	}
	return result.URL, nil
}
