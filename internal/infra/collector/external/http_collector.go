package external

import (
	domaincfg "github.com/youxihu/GWatch-new/internal/domain/config"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPResponseBody HTTP响应体结构
type HTTPResponseBody struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// HTTPCollector HTTP接口监控收集器
type HTTPCollector struct {
	provider domaincfg.Provider
	client   *http.Client
}

// NewHTTPCollector 创建新的HTTP接口监控收集器
func NewHTTPCollector(p domaincfg.Provider) *HTTPCollector {
	return &HTTPCollector{provider: p}
}

// Init 初始化HTTP客户端
func (c *HTTPCollector) Init() error {
	// 创建HTTP客户端，不设置固定超时（超时由每次请求的context控制）
	// 这样可以支持每个接口配置不同的超时时间
	c.client = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	return nil
}

// CheckInterface 检查指定HTTP接口的可访问性和响应时间
func (c *HTTPCollector) CheckInterface(url string, timeout time.Duration) (bool, time.Duration, int, error) {
	if c.client == nil {
		return false, 0, 0, fmt.Errorf("HTTP客户端未初始化")
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, 0, 0, fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置User-Agent避免被某些服务拒绝
	req.Header.Set("User-Agent", "GWatch-Monitor/1.0")

	// 记录开始时间
	start := time.Now()

	// 发送请求
	resp, err := c.client.Do(req)
	if err != nil {
		// 检查是否是超时错误
		if ctx.Err() == context.DeadlineExceeded {
			return false, 0, 0, fmt.Errorf("HTTP请求超时: 超过 %v", timeout)
		}
		// 网络错误、连接超时等，接口不可访问
		return false, 0, 0, fmt.Errorf("HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 计算响应时间
	responseTime := time.Since(start)

	// 获取HTTP状态码
	httpStatusCode := resp.StatusCode

	// 尝试解析响应体，获取业务状态码
	var businessStatusCode int
	var businessError error

	// 读取响应体
	bodyBytes, err := io.ReadAll(resp.Body)
	if err == nil && len(bodyBytes) > 0 {
		var responseBody HTTPResponseBody
		if json.Unmarshal(bodyBytes, &responseBody) == nil {
			businessStatusCode = responseBody.Code
			// 如果业务状态码不是200，说明有业务错误
			if businessStatusCode != 200 {
				businessError = fmt.Errorf("业务错误: %s (code: %d)", responseBody.Msg, businessStatusCode)
			}
		}
	}

	// 优先使用业务状态码，如果没有则使用HTTP状态码
	finalStatusCode := businessStatusCode
	if finalStatusCode == 0 {
		finalStatusCode = httpStatusCode
	}

	// 如果HTTP状态码是200但业务状态码表示错误，返回false
	if httpStatusCode == 200 && businessError != nil {
		return false, responseTime, finalStatusCode, businessError
	}

	// 只要能够收到HTTP响应，就认为接口是可访问的
	return true, responseTime, finalStatusCode, nil
}

// Close 释放资源
func (c *HTTPCollector) Close() {
	if c.client != nil {
		c.client.CloseIdleConnections()
	}
}
