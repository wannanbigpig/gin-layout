package sms

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type AliyunProvider struct {
	config AliyunConfig
	client *http.Client
	now    func() time.Time
	nonce  func() string
}

func NewAliyunProvider(raw json.RawMessage) (Provider, error) {
	var cfg AliyunConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("阿里云短信配置格式错误: %w", err)
	}
	if !cfg.Enabled {
		return nil, errors.New("阿里云短信供应商未启用")
	}
	if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.AccessKeySecret) == "" {
		return nil, errors.New("阿里云短信 AccessKey 未配置")
	}
	if strings.TrimSpace(cfg.Endpoint) == "" {
		cfg.Endpoint = "https://dysmsapi.aliyuncs.com"
	}
	if cfg.Endpoint != "https://dysmsapi.aliyuncs.com" {
		return nil, errors.New("阿里云短信 Endpoint 必须是完整官方 URL https://dysmsapi.aliyuncs.com，不能填写 Region ID")
	}
	return &AliyunProvider{
		config: cfg,
		client: &http.Client{Timeout: 8 * time.Second},
		now:    time.Now,
		nonce:  uuid.NewString,
	}, nil
}

type aliyunSendResponse struct {
	Code      string `json:"Code"`
	Message   string `json:"Message"`
	RequestID string `json:"RequestId"`
	BizID     string `json:"BizId"`
}

func (p *AliyunProvider) Send(ctx context.Context, input SendInput) (SendResult, error) {
	if p == nil {
		return SendResult{}, errors.New("阿里云短信供应商未初始化")
	}
	templateParam, err := json.Marshal(input.Variables)
	if err != nil {
		return SendResult{}, err
	}
	params := map[string]string{
		"AccessKeyId":      p.config.AccessKeyID,
		"Action":           "SendSms",
		"Format":           "JSON",
		"PhoneNumbers":     strings.TrimPrefix(input.PhoneNumber, "+86"),
		"RegionId":         "cn-hangzhou",
		"SignName":         input.SignName,
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   p.nonce(),
		"SignatureVersion": "1.0",
		"TemplateCode":     input.TemplateCode,
		"TemplateParam":    string(templateParam),
		"Timestamp":        p.now().UTC().Format("2006-01-02T15:04:05Z"),
		"Version":          "2017-05-25",
	}
	params["Signature"] = aliyunSignature(params, p.config.AccessKeySecret)
	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.Endpoint+"/", strings.NewReader(values.Encode()))
	if err != nil {
		return SendResult{}, errors.New("创建阿里云短信请求失败")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.client.Do(req)
	if err != nil {
		// http.Client.Do 返回的 url.Error 会包含请求 URL；禁止把传输层错误向上包装，
		// 避免未来请求参数调整后在日志中泄露手机号、验证码、AccessKey 或签名。
		return SendResult{State: SendStateUnknown}, errors.New("阿里云短信请求状态未知")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return SendResult{State: SendStateUnknown}, errors.New("读取阿里云短信响应失败")
	}
	var payload aliyunSendResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return SendResult{State: SendStateUnknown}, fmt.Errorf("阿里云短信响应格式错误")
	}
	result := SendResult{ProviderRequestID: payload.RequestID, ProviderMessageID: payload.BizID, ProviderCode: payload.Code}
	if resp.StatusCode != http.StatusOK || payload.Code != "OK" {
		result.State = SendStateFailed
		return result, errors.New("阿里云短信发送失败")
	}
	result.State = SendStateAccepted
	return result, nil
}

func aliyunSignature(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, aliyunPercentEncode(key)+"="+aliyunPercentEncode(params[key]))
	}
	canonical := strings.Join(parts, "&")
	stringToSign := "POST&%2F&" + aliyunPercentEncode(canonical)
	mac := hmac.New(sha1.New, []byte(secret+"&"))
	_, _ = mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func aliyunPercentEncode(value string) string {
	escaped := url.QueryEscape(value)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	escaped = strings.ReplaceAll(escaped, "*", "%2A")
	escaped = strings.ReplaceAll(escaped, "%7E", "~")
	return escaped
}
