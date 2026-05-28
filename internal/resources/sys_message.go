package resources

import (
	"encoding/json"
	"strings"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// SysMessageBaseResources 表示系统消息公共响应字段。
type SysMessageBaseResources struct {
	ID          uint              `json:"id"`
	Category    string            `json:"category"`
	Topic       string            `json:"topic"`
	Title       string            `json:"title"`
	Content     string            `json:"content"`
	Payload     map[string]any    `json:"payload"`
	ActionURL   string            `json:"action_url"`
	ActionLabel string            `json:"action_label"`
	IsRead      bool              `json:"is_read"`
	ReadAt      *utils.FormatDate `json:"read_at"`
	CreatedAt   utils.FormatDate  `json:"created_at"`
	UpdatedAt   utils.FormatDate  `json:"updated_at"`
}

// SysMessageManageListResources 表示管理员通知管理列表项。
type SysMessageManageListResources struct {
	BatchID         string            `json:"batch_id"`
	SenderID        uint              `json:"sender_id"`
	Category        string            `json:"category"`
	Title           string            `json:"title"`
	Content         string            `json:"content"`
	ActionURL       string            `json:"action_url"`
	ActionLabel     string            `json:"action_label"`
	RecipientType   string            `json:"recipient_type"`
	RecipientIDs    []uint            `json:"recipient_ids"`
	RecipientCount  int64             `json:"recipient_count"`
	SendImmediately bool              `json:"send_immediately"`
	SendAt          *utils.FormatDate `json:"send_at"`
	SentAt          *utils.FormatDate `json:"sent_at"`
	Status          string            `json:"status"`
	CreatedAt       utils.FormatDate  `json:"created_at"`
	UpdatedAt       utils.FormatDate  `json:"updated_at"`
}

// ManualSendResultResources 表示管理员发送通知结果。
type ManualSendResultResources struct {
	BatchID         string            `json:"batch_id"`
	Category        string            `json:"category"`
	Title           string            `json:"title"`
	Content         string            `json:"content"`
	ActionURL       string            `json:"action_url"`
	ActionLabel     string            `json:"action_label"`
	RecipientType   string            `json:"recipient_type"`
	RecipientIDs    []uint            `json:"recipient_ids"`
	RecipientCount  int64             `json:"recipient_count"`
	SendImmediately bool              `json:"send_immediately"`
	SendAt          *utils.FormatDate `json:"send_at"`
	SentAt          *utils.FormatDate `json:"sent_at"`
	Status          string            `json:"status"`
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
		ID:          data.ID,
		Category:    strings.TrimSpace(data.Category),
		Topic:       strings.TrimSpace(data.Topic),
		Title:       strings.TrimSpace(data.Title),
		Content:     strings.TrimSpace(data.Content),
		Payload:     decodeSysMessagePayload(data.Payload),
		ActionURL:   resolveSysMessageActionURL(data),
		ActionLabel: strings.TrimSpace(data.ActionLabel),
		IsRead:      data.IsRead == model.MessageRead,
		ReadAt:      data.ReadAt,
		CreatedAt:   data.CreatedAt,
		UpdatedAt:   data.UpdatedAt,
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

func resolveSysMessageActionURL(data *model.SysMessage) string {
	if url := strings.TrimSpace(data.ActionURL); url != "" {
		return url
	}
	payload := decodeSysMessagePayload(data.Payload)
	if url, ok := payload["download_url"].(string); ok {
		url = strings.TrimSpace(url)
		if url != "" {
			return url
		}
	}
	if url, ok := payload["action_url"].(string); ok {
		return strings.TrimSpace(url)
	}
	return ""
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

func DecodeSysMessageRecipientIDs(raw string) []uint {
	if strings.TrimSpace(raw) == "" {
		return []uint{}
	}
	var ids []uint
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return []uint{}
	}
	return ids
}
