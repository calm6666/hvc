package auth

import "net/http"

// LoginUseCase 表示登录用例。
type LoginUseCase struct{}

// Execute 执行登录。
func (u *LoginUseCase) Execute(r *http.Request) bool {
	return r != nil
}
