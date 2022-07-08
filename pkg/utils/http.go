package utils

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
)

type HttpRequest struct {
	http.Client
	Response *http.Response
	Error    error
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

	return ioutil.ReadAll(hr.Response.Body)
}
