package config

import (
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
)

// Provider 定义配置提供者的领域接口
// 仅暴露读取能力，加载与来源由基础设施层实现
type Provider interface {
	GetConfig() *shared.Config
}
