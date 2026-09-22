package form

import (
	"encoding/json"

	"github.com/wannanbigpig/gin-layout/internal/sms"
)

type SMSConfigPayload struct {
	Enabled   bool                               `form:"enabled" json:"enabled"`
	Providers map[string]json.RawMessage         `form:"providers" json:"providers" label:"短信供应商" binding:"required"`
	Templates map[string]map[string]sms.Template `form:"templates" json:"templates" label:"短信模板" binding:"required"`
	Routes    map[string]sms.Route               `form:"routes" json:"routes" label:"场景路由" binding:"required"`
	OTP       sms.OTPConfig                      `form:"otp" json:"otp" label:"验证码配置" binding:"required"`
}

func NewSMSConfigPayload() *SMSConfigPayload { return &SMSConfigPayload{} }

func (p *SMSConfigPayload) ToConfig() sms.Config {
	return sms.Config{Enabled: p.Enabled, Providers: p.Providers, Templates: p.Templates, Routes: p.Routes, OTP: p.OTP}
}

type SMSTestPayload struct {
	CountryCode string            `form:"country_code" json:"country_code" label:"国家代码" binding:"required,oneof=86 +86"`
	PhoneNumber string            `form:"phone_number" json:"phone_number" label:"手机号" binding:"required,phone_number"`
	Config      *SMSConfigPayload `form:"config" json:"config" label:"当前短信配置" binding:"required"`
}

func NewSMSTestPayload() *SMSTestPayload { return &SMSTestPayload{} }

type SMSCodePayload struct {
	CountryCode string `form:"country_code" json:"country_code" label:"国家代码" binding:"required,oneof=86 +86"`
	PhoneNumber string `form:"phone_number" json:"phone_number" label:"手机号" binding:"required,phone_number"`
	Captcha     string `form:"captcha" json:"captcha" label:"验证码" binding:"required"`
	CaptchaID   string `form:"captcha_id" json:"captcha_id" binding:"required"`
}

func NewSMSCodePayload() *SMSCodePayload { return &SMSCodePayload{} }

type SMSLoginPayload struct {
	CountryCode string `form:"country_code" json:"country_code" label:"国家代码" binding:"required,oneof=86 +86"`
	PhoneNumber string `form:"phone_number" json:"phone_number" label:"手机号" binding:"required,phone_number"`
	ChallengeID string `form:"challenge_id" json:"challenge_id" label:"短信验证标识" binding:"required,uuid"`
	SMSCode     string `form:"sms_code" json:"sms_code" label:"短信验证码" binding:"required,len=6,numeric"`
}

func NewSMSLoginPayload() *SMSLoginPayload { return &SMSLoginPayload{} }
