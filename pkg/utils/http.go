package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type HttpRequest struct {
	http.Client
	Response *http.Response
	Error    error
}

// JsonRequest 默认 Content-Type：application/json 类型请求
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

// GetRequest 发起 GET 请求
func (hr *HttpRequest) GetRequest(url string, params *url.Values, args ...any) *HttpRequest {
	r := url
	if params != nil {
		r = fmt.Sprintf("%s?%s", url, params.Encode())
	}

	return hr.Request("GET", r, nil, args...)
}

// Request make a request
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

// ParseJson Parse the return value into json format
func (hr *HttpRequest) ParseJson(payload any) error {
	bytes, err := hr.ParseBytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, &payload)
}

// ParseBytes Parse the return value into []byte format
func (hr *HttpRequest) ParseBytes() ([]byte, error) {
	if hr.Error != nil {
		return nil, hr.Error
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			panic(err.Error())
		}
	}(hr.Response.Body)

	return io.ReadAll(hr.Response.Body)
}

// Raw Return the raw response data as a string
func (hr *HttpRequest) Raw() (string, error) {
	str, err := hr.ParseBytes()
	if err != nil {
		return "", err
	}
	return string(str), nil
}
