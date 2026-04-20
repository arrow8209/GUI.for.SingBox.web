package bridge

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"

	"guiforcores/pkg/security"
)

// GetPath 把 relPath 拼到 Env.BasePath 下，但限制必须落在 data/ 子树内。
// 拒绝绝对路径、`..` 越界、symlink 越狱以及 data/ 之外的子目录。
// 安全失败时返回空串，调用方需检查并降级处理。
func GetPath(relPath string) string {
	sb := security.NewSandbox(Env.BasePath)
	full, err := sb.Resolve(relPath)
	if err != nil {
		log.Printf("GetPath sandbox rejected %q: %v", relPath, err)
		return ""
	}
	dataPath := filepath.Join(sb.Base(), "data")
	if full != dataPath && !strings.HasPrefix(full, dataPath+string(filepath.Separator)) {
		log.Printf("GetPath rejected %q: outside data/ subtree", relPath)
		return ""
	}
	return filepath.ToSlash(full)
}

func GetProxy(_proxy string) func(*http.Request) (*url.URL, error) {
	proxy := http.ProxyFromEnvironment

	if _proxy != "" {
		proxyUrl, err := url.Parse(_proxy)
		if err == nil {
			proxy = http.ProxyURL(proxyUrl)
		}
	}

	return proxy
}

func GetTimeout(timeout int) time.Duration {
	if timeout <= 0 {
		return 15 * time.Second
	}
	return time.Duration(timeout) * time.Second
}

func GetHeader(headers map[string]string) http.Header {
	header := make(http.Header, len(headers))
	for key, value := range headers {
		header.Set(key, value)
	}
	return header
}

func ConvertByte2String(byte []byte) string {
	decodeBytes, _ := simplifiedchinese.GB18030.NewDecoder().Bytes(byte)
	return string(decodeBytes)
}

func ParseRange(s string, size int64) (start int64, end int64, err error) {
	if s == "" {
		return 0, size - 1, nil
	}

	s = strings.TrimSpace(s)

	// "bytes=100-200"
	s = strings.TrimPrefix(s, "bytes=")

	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, errors.New("invalid range format")
	}

	startStr := strings.TrimSpace(parts[0])
	endStr := strings.TrimSpace(parts[1])

	// "-200" last 200 bytes
	if startStr == "" && endStr != "" {
		e, err2 := strconv.ParseInt(endStr, 10, 64)
		if err2 != nil || e < 0 {
			return 0, 0, errors.New("invalid range value")
		}
		if e > size {
			start = 0
		} else {
			start = size - e
		}
		end = size - 1
		return start, end, nil
	}

	// "100-" from start to EOF
	if startStr != "" && endStr == "" {
		start, err = strconv.ParseInt(startStr, 10, 64)
		if err != nil || start < 0 {
			return 0, 0, errors.New("invalid range value")
		}
		end = size - 1
		return start, end, nil
	}

	// "100-200"
	if startStr != "" && endStr != "" {
		start, err = strconv.ParseInt(startStr, 10, 64)
		if err != nil || start < 0 {
			return 0, 0, errors.New("invalid range value")
		}
		end, err = strconv.ParseInt(endStr, 10, 64)
		if err != nil || end < 0 {
			return 0, 0, errors.New("invalid range value")
		}
		if start > end {
			return 0, 0, errors.New("invalid range: start > end")
		}
		if end >= size {
			end = size - 1
		}
		return start, end, nil
	}

	return 0, 0, errors.New("invalid range format")
}

func RollingRelease(next http.Handler) http.Handler {
	isDevVersion := strings.Contains(Env.AppVersion, "dev")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Path
		isIndex := url == "/"

		if isIndex {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "max-age=31536000, immutable")
		}

		if isDevVersion || !Config.RollingRelease {
			next.ServeHTTP(w, r)
			return
		}

		if isIndex {
			url = "/index.html"
		}

		filePath := GetPath("data/rolling-release" + url)
		if _, err := os.Stat(filePath); err != nil {
			next.ServeHTTP(w, r)
			return
		}

		http.ServeFile(w, r, filePath)
	})
}
