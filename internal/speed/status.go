package speed

import (
	"context"
	"errors"
)

// StatusError carries the original application's status code and public text.
// Cause is retained for diagnostics, never included in the public message.
type StatusError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Cause   error  `json:"-"`
}

func (e *StatusError) Error() string { return e.Message }
func (e *StatusError) Unwrap() error { return e.Cause }
func Failure(code int, cause error) *StatusError {
	names := map[int]string{121: "云服务器连接失败", 122: "云服务器响应错误", 123: "没匹配到列表中服务器", 130: "测速服务器连接失败", 131: "测速服务器无响应", 132: "测速服务器繁忙,请稍后重试", 133: "测速服务器忙，请稍后再试", 134: "请求排队参数错误", 135: "测速服务器或网络异常", 136: "参数设置错误", 137: "下行测试无响应", 138: "上行测试无响应", 139: "当前未选择测速服务器", 990: "用户中止测试", 992: "正在测试中", 999: "服务器或网络异常"}
	message, ok := names[code]
	if !ok {
		message = "其他原因失败"
	}
	return &StatusError{Code: code, Message: message, Cause: cause}
}
func PublicError(err error) *StatusError {
	if errors.Is(err, context.Canceled) {
		return Failure(990, err)
	}
	var status *StatusError
	if errors.As(err, &status) {
		return status
	}
	return Failure(999, err)
}
