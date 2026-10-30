// inspect.go 是模型池巡检与清单对账（FR-408/FR-1103）。
//
// @file internal/gateway/inspect.go
// 文件职责：以 GET {base}/v1/models 的实际返回为对账基准，核对 models.json 里
// enabled 的模型；连续 2 次对不上 → 从下拉剔除（health 置 degraded）、记 WARN；
// 恢复后自动回来。巡检由 cmd/server 在启动时与每小时触发（间隔可经
// FR_INSPECT_INTERVAL_SECONDS 覆盖，供测试缩短）。
//
// 主要内容：Pool.Inspect、Pool.ModelHealth、Pool.AvailableModels。
//
// 模块边界：只依赖 Client（拉模型清单）与 Catalog（展示清单）；不改配置文件。
package gateway

import (
	"log/slog"
)

// inspectFailThreshold 是连续失败多少次后把模型剔除出下拉（FR-1103）。
const inspectFailThreshold = 2

// Inspect 执行一次对账。live 为上游实际可调用的模型 id 集合（nil 表示本次拉取失败，
// 保留现状不剔除——网络抖动不应立刻把全部模型下架）。
func (p *Pool) Inspect(live map[string]bool) {
	if p.catalog == nil {
		return
	}
	p.modelMu.Lock()
	defer p.modelMu.Unlock()

	anyDegraded := false
	for _, m := range p.catalog.Enabled() {
		if live == nil {
			// 拉取失败：不累计（避免误杀），但也无法确认健康。
			continue
		}
		if live[m.ID] {
			if p.modelFails[m.ID] > 0 {
				slog.Info("模型恢复，重新进入下拉", "model", m.ID)
			}
			p.modelFails[m.ID] = 0
			continue
		}
		p.modelFails[m.ID]++
		if p.modelFails[m.ID] >= inspectFailThreshold {
			slog.Warn("模型连续对账失败，已从下拉剔除", "model", m.ID, "fails", p.modelFails[m.ID])
			anyDegraded = true
		} else {
			slog.Warn("模型对账失败", "model", m.ID, "fails", p.modelFails[m.ID])
		}
	}
	p.SetDegraded(anyDegraded)
}

// ModelHealth 返回某模型的巡检状态：ok / degraded（连续失败 1 次）/
// removed（连续失败 ≥2 次，从下拉剔除）。清单里没有的模型返回 ok。
func (p *Pool) ModelHealth(id string) string {
	p.modelMu.Lock()
	defer p.modelMu.Unlock()
	switch {
	case p.modelFails[id] >= inspectFailThreshold:
		return "removed"
	case p.modelFails[id] > 0:
		return "degraded"
	default:
		return "ok"
	}
}

// AvailableModels 返回仍在下拉里的 enabled 模型 id（被剔除的不算）。
func (p *Pool) AvailableModels() []string {
	if p.catalog == nil {
		return nil
	}
	p.modelMu.Lock()
	defer p.modelMu.Unlock()
	out := make([]string, 0, len(p.catalog.Enabled()))
	for _, m := range p.catalog.Enabled() {
		if p.modelFails[m.ID] < inspectFailThreshold {
			out = append(out, m.ID)
		}
	}
	return out
}
