package resources

import (
	"encoding/json"
	"strings"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// SysMessageBaseResources 表示系统消息公共响应字段。
type SysMessageBaseResources struct {
	ID        uint              `json:"id"`
	Category  string            `json:"category"`
	Topic     string            `json:"topic"`
	Title     string            `json:"title"`
	Content   string            `json:"content"`
	Payload   map[string]any    `json:"payload"`
	IsRead    bool              `json:"is_read"`
	ReadAt    *utils.FormatDate `json:"read_at"`
	CreatedAt utils.FormatDate  `json:"created_at"`
	UpdatedAt utils.FormatDate  `json:"updated_at"`
}

// SysMessageListResources 表示系统消息列表项。
type SysMessageListResources struct {
	SysMessageBaseResources
}

// SysMessageResources 表示系统消息详情。
type SysMessageResources struct {
	SysMessageBaseResources
}

type SysMessageTransformer struct {
	BaseResources[*model.SysMessage, *SysMessageResources]
}

func NewSysMessageTransformer() SysMessageTransformer {
	return SysMessageTransformer{
		BaseResources: BaseResources[*model.SysMessage, *SysMessageResources]{
			NewResource: func() *SysMessageResources {
				return &SysMessageResources{}
			},
		},
	}
}

func buildSysMessageBaseResources(data *model.SysMessage) SysMessageBaseResources {
	return SysMessageBaseResources{
		ID:        data.ID,
		Category:  strings.TrimSpace(data.Category),
		Topic:     strings.TrimSpace(data.Topic),
		Title:     strings.TrimSpace(data.Title),
		Content:   strings.TrimSpace(data.Content),
		Payload:   decodeSysMessagePayload(data.Payload),
		IsRead:    data.IsRead == model.MessageRead,
		ReadAt:    data.ReadAt,
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
	}
}

func (r SysMessageTransformer) ToStruct(data *model.SysMessage) *SysMessageResources {
	return &SysMessageResources{SysMessageBaseResources: buildSysMessageBaseResources(data)}
}

func (r SysMessageTransformer) ToCollection(page, perPage int, total int64, data []*model.SysMessage) *Collection {
	items := make([]any, 0, len(data))
	for _, item := range data {
		items = append(items, &SysMessageListResources{
			SysMessageBaseResources: buildSysMessageBaseResources(item),
		})
	}
	return NewCollection().SetPaginate(page, perPage, total).ToCollection(items)
}

func decodeSysMessagePayload(raw string) map[string]any {
	payload := make(map[string]any)
	if strings.TrimSpace(raw) == "" {
		return payload
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return map[string]any{}
	}
	return payload
}
