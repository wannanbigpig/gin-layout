package utils

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
)

// HttpRequest 封装带状态的 HTTP 请求客户端。
type HttpRequest struct {
	http.Client
	Response *http.Response
	Error    error
}

// JsonRequest 发送默认 Content-Type 为 application/json 的请求。
func (hr *HttpRequest) JsonRequest(method string, url string, body io.Reader, args ...any) *HttpRequest {
	var options map[string]string
	if args != nil {
		var ok bool
		if options, ok = args[0].(map[string]string); ok {
			options["Content-Type"] = "application/json"
		}
	} else {
		options = map[string]string{
			"Content-Type": "application/json",
		}
	}
	return hr.Request(method, url, body, options)
}

// GetRequest 发送 GET 请求并拼接查询参数。
func (hr *HttpRequest) GetRequest(url string, params *url.Values, args ...any) *HttpRequest {
	r := url
	if params != nil {
		r = url + "?" + params.Encode()
	}

	return hr.Request("GET", r, nil, args...)
}

// Request 构造并发送 HTTP 请求。
func (hr *HttpRequest) Request(method string, url string, body io.Reader, args ...any) *HttpRequest {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		hr.Error = err
	}

	if args != nil {
		if options, ok := args[0].(map[string]string); ok {
			for k, v := range options {
				req.Header.Set(k, v)
			}
		}
	}

	hr.Response, hr.Error = hr.Do(req)

	return hr
}

// ParseJson 将响应体解析为目标 JSON 结构。
func (hr *HttpRequest) ParseJson(payload any) error {
	bytes, err := hr.ParseBytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, &payload)
}

// ParseBytes 读取并返回原始响应体字节。
func (hr *HttpRequest) ParseBytes() (body []byte, err error) {
	if hr.Error != nil {
		return nil, hr.Error
	}
	if hr.Response == nil || hr.Response.Body == nil {
		return nil, errors.New("http response body is nil")
	}

	defer func() {
		if closeErr := hr.Response.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	body, err = io.ReadAll(hr.Response.Body)
	return body, err
}

// Raw 以字符串形式返回原始响应体。
func (hr *HttpRequest) Raw() (string, error) {
	str, err := hr.ParseBytes()
	if err != nil {
		return "", err
	}
	return string(str), nil
}
