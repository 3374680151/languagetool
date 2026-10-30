// model.go 定义 §7.5 的全部 GORM 模型。
//
// @file internal/store/model.go
// 文件职责：七张表的结构定义——visitors（匿名访客）、usage_daily（访客日用量，
// 只有数字）、site_usage_daily（站点日用量，只有数字）、glossaries（术语）、
// words（词表+学习字段）、mistakes（错误本片段）、user_prefs（偏好 JSON）。
//
// 主要内容：七个结构体及其索引/唯一约束声明。
//
// 模块边界：只描述结构，不含查询；查询在 *_repo.go。
// 隐私红线（NFR-6）：没有一张表存正文/译文；words.context_sentence 与
// mistakes 的两个 snippet 是用户显式动作（收藏/接受纠错）写入的片段级例外。
package store

import "time"

// Visitor 是匿名访客身份（FR-1104）：UUID 主键，只存时间，不存任何个人信息。
type Visitor struct {
	ID         string    `gorm:"primaryKey;size:36" json:"id"`
	CreatedAt  time.Time `json:"createdAt"`
	LastSeenAt time.Time `json:"lastSeenAt"`
}

// UsageDaily 是某访客某天的用量，UNIQUE(visitor_id, date)。
// 只有数字（NFR-6）：段数、字符数、失败数、总耗时——没有一段正文。
type UsageDaily struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"-"`
	VisitorID string    `gorm:"size:36;uniqueIndex:ux_visitor_day,priority:1" json:"-"`
	Date      string    `gorm:"size:10;uniqueIndex:ux_visitor_day,priority:2" json:"date"`
	Segments  int64     `json:"segments"`
	Chars     int64     `json:"chars"`
	Failures  int64     `json:"failures"`
	TotalMS   int64     `json:"totalMs"`
	UpdatedAt time.Time `json:"-"`
}

// SiteUsageDaily 是站点级日用量，date 为主键（FR-603/604）。
// paid_chars 是封顶依据：只有站点付费线路成功的字符才累计。
type SiteUsageDaily struct {
	Date         string `gorm:"primaryKey;size:10" json:"date"`
	PaidChars    int64  `json:"paidChars"`
	FreeChars    int64  `json:"freeChars"`
	TranslateReq int64  `json:"translateReq"`
	ProofreadReq int64  `json:"proofreadReq"`
	DegradedReq  int64  `json:"degradedReq"`
}

// Glossary 是一条术语（FR-501/505）。scope=site 全站共享；scope=user 随访客。
// 冲突按「站点 > 用户」消解，由仓储查询层保证。
type Glossary struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Scope       string    `gorm:"size:8;index" json:"scope"`
	VisitorID   string    `gorm:"size:36;index" json:"visitorId,omitempty"`
	SourceTerm  string    `gorm:"size:200;uniqueIndex:ux_site_term,priority:1,where:scope='site'" json:"sourceTerm"`
	TargetTerm  string    `gorm:"size:200" json:"targetTerm"`
	Note        string    `gorm:"size:500" json:"note,omitempty"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Word 是一条收藏词（FR-702/1105/1202/1204）。
// context_sentence 是用户显式收藏时带来的原句片段（学习资产例外，NFR-6）。
// use_count/last_used_at 服务"再用标记"（FR-1202）；next_due_at/stage 服务挖空复习（FR-1204）。
type Word struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	VisitorID      string    `gorm:"size:36;index:ux_visitor_word,priority:1,unique" json:"-"`
	Word           string    `gorm:"size:100;index:ux_visitor_word,priority:2,unique" json:"word"`
	Lang           string    `gorm:"size:12" json:"lang"`
	ContextSentence string   `gorm:"size:300" json:"contextSentence,omitempty"`
	SourceURL      string    `gorm:"size:500" json:"sourceUrl,omitempty"`
	UseCount       int64     `json:"useCount"`
	LastUsedAt     *time.Time `json:"lastUsedAt,omitempty"`
	NextDueAt      *time.Time `json:"nextDueAt,omitempty"`
	Stage          int       `json:"-"`
	CreatedAt      time.Time `json:"createdAt"`
}

// Mistake 是一条错误本记录（FR-1203/1204）：只在用户"接受"纠错建议时写入片段。
// 存片段（原片段→改法），不存整段正文（NFR-6 例外边界）。
type Mistake struct {
	ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	VisitorID        string    `gorm:"size:36;index" json:"-"`
	SourceSnippet    string    `gorm:"size:300" json:"sourceSnippet"`
	CorrectedSnippet string    `gorm:"size:300" json:"correctedSnippet"`
	Category         string    `gorm:"size:20;index" json:"category"`
	Reason           string    `gorm:"size:500" json:"reason"`
	NextDueAt        *time.Time `json:"nextDueAt,omitempty"`
	Stage            int       `json:"-"`
	CreatedAt        time.Time `json:"createdAt"`
}

// UserPref 是一条访客偏好（FR-1001 的 6 项），复合主键 (visitor_id, key)。
// value 存 JSON 字符串，读出后由 service 解释为具体类型。
type UserPref struct {
	VisitorID string `gorm:"primaryKey;size:36" json:"-"`
	Key       string `gorm:"primaryKey;size:50" json:"key"`
	Value     string `gorm:"size:1000" json:"value"`
	UpdatedAt time.Time `json:"-"`
}

// TableName 显式指定表名（§7.5）：GORM 默认会把 Daily 复数成 dailies，
// 与规格的表名不符，这里固定住。
func (UsageDaily) TableName() string { return "usage_daily" }

// TableName 同上。
func (SiteUsageDaily) TableName() string { return "site_usage_daily" }
