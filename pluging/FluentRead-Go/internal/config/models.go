// models.go 负责加载并校验"模型展示清单"deploy/models.json（FR-402/408/409）。
//
// @file internal/config/models.go
// 文件职责：解析清单文件（id/label/role/quality/speed/cost/enabled），启动校验
// （enabled 的模型必须有 id，模板占位模型不得启用），并提供 primary 与可选池两类查询。
// 清单只描述"展示与角色"，实际可调用性以网关 /v1/models 对账为准（FR-1103）。
//
// 主要内容：ModelInfo、Catalog、LoadCatalog、Primary、Enabled、Allowed。
//
// 模块边界：只依赖标准库；被 cmd/server（启动校验）与 httpapi（/api/config 下发）使用；
// 不感知网络与数据库。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// 模型角色。role:"primary" 且 enabled 的模型是 L1 默认线路；其余 enabled 的是 L2 站点池。
const (
	RolePrimary = "primary"
	RolePool    = "pool"
)

// ModelInfo 是清单里的一条模型。Enabled 决定是否出现在下拉里。
type ModelInfo struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Role    string `json:"role,omitempty"`
	Quality string `json:"quality,omitempty"`
	Speed   string `json:"speed,omitempty"`
	Cost    string `json:"cost,omitempty"`
	Enabled bool   `json:"enabled"`
}

// Catalog 是模型清单。
type Catalog struct {
	models []ModelInfo
}

// LoadCatalog 读取清单文件。文件不存在时返回空清单而不是错误——
// 没配模型也应该能启动，站点处于六态之"未配置"（§5.2）。
func LoadCatalog(path string) (*Catalog, error) {
	if path == "" {
		return &Catalog{}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Catalog{}, nil
		}
		return nil, fmt.Errorf("读取模型清单失败: %w", err)
	}

	var models []ModelInfo
	if err := json.Unmarshal(raw, &models); err != nil {
		return nil, fmt.Errorf("模型清单格式错误: %w", err)
	}

	if err := validate(models); err != nil {
		return nil, err
	}

	// 去重，后出现的覆盖先出现的，便于在清单里覆盖某条。
	seen := make(map[string]int, len(models))
	cleaned := make([]ModelInfo, 0, len(models))
	for _, m := range models {
		if m.ID == "" {
			continue
		}
		if idx, ok := seen[m.ID]; ok {
			cleaned[idx] = m
			continue
		}
		seen[m.ID] = len(cleaned)
		cleaned = append(cleaned, m)
	}
	return &Catalog{models: cleaned}, nil
}

// validate 是 FR-409 的启动校验：enabled 的模型必须有真实 id；
// 占位模板（id 形如 my-custom / your-model-name）不允许启用，
// 否则下拉里会出现一个调不通的模型，访客选完必失败。
func validate(models []ModelInfo) error {
	for _, m := range models {
		if !m.Enabled {
			continue
		}
		if strings.TrimSpace(m.ID) == "" {
			return fmt.Errorf("清单里有 enabled 的模型缺少 id，请补全或改为 enabled:false")
		}
		id := strings.ToLower(m.ID)
		for _, placeholder := range []string{"my-custom", "your-model", "your-model-name", "example", "placeholder"} {
			if id == placeholder {
				return fmt.Errorf("模型 %q 是占位模板，启用前必须填真实模型名（FR-409）", m.ID)
			}
		}
	}
	return nil
}

// Primary 返回 L1 默认模型：role=primary 且 enabled 的第一项。
func (c *Catalog) Primary() (ModelInfo, bool) {
	if c == nil {
		return ModelInfo{}, false
	}
	for _, m := range c.models {
		if m.Enabled && m.Role == RolePrimary {
			return m, true
		}
	}
	return ModelInfo{}, false
}

// Enabled 返回对访客开放的全部模型（L1 primary + L2 池），顺序即清单顺序。
func (c *Catalog) Enabled() []ModelInfo {
	if c == nil {
		return nil
	}
	out := make([]ModelInfo, 0, len(c.models))
	for _, m := range c.models {
		if m.Enabled {
			out = append(out, m)
		}
	}
	return out
}

// Allowed 判断某个模型是否对访客开放。
func (c *Catalog) Allowed(id string) bool {
	if c == nil || id == "" {
		return false
	}
	for _, m := range c.models {
		if m.ID == id && m.Enabled {
			return true
		}
	}
	return false
}

// Count 返回清单总条数，用于启动日志。
func (c *Catalog) Count() int {
	if c == nil {
		return 0
	}
	return len(c.models)
}
