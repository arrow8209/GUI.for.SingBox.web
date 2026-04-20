package security

import (
	"errors"
	"path/filepath"
	"strings"
)

// ErrSandboxEscape 表示路径越出 sandbox 根。
var ErrSandboxEscape = errors.New("path escapes sandbox")

// Sandbox 限制路径只能在 base 子树内。
type Sandbox struct {
	base string
}

// NewSandbox 用绝对化 + 清理后的 base 构造 sandbox。
func NewSandbox(base string) *Sandbox {
	abs, err := filepath.Abs(base)
	if err != nil {
		abs = base
	}
	return &Sandbox{base: filepath.Clean(abs)}
}

// Resolve 把 rel 拼到 base 下，返回绝对路径。
// 拒绝绝对路径、`..`、symlink 越狱。
// 空 rel 返回 base 本身（用于"列根目录"场景）。
func (s *Sandbox) Resolve(rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", ErrSandboxEscape
	}
	cleaned := filepath.Clean(rel)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", ErrSandboxEscape
	}
	full := filepath.Join(s.base, cleaned)
	// 防止边界绕过：拼接后必须仍以 base 开头
	if full != s.base && !strings.HasPrefix(full, s.base+string(filepath.Separator)) {
		return "", ErrSandboxEscape
	}
	// Symlink 校验：如果路径已存在，EvalSymlinks 后必须仍在 base 内
	if resolved, err := filepath.EvalSymlinks(full); err == nil {
		if resolved != s.base && !strings.HasPrefix(resolved, s.base+string(filepath.Separator)) {
			return "", ErrSandboxEscape
		}
	}
	// 父目录如果存在也校验 symlink，避免路径不存在场景被绕过
	if dir := filepath.Dir(full); dir != s.base {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			if resolved != s.base && !strings.HasPrefix(resolved, s.base+string(filepath.Separator)) {
				return "", ErrSandboxEscape
			}
		}
	}
	return full, nil
}

// Base 返回 sandbox 根（绝对路径）。
func (s *Sandbox) Base() string {
	return s.base
}
