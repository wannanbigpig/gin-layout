package utils

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGetRequest(t *testing.T) {
	client := HttpRequest{}
	client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("name") != "world" {
			t.Fatalf("unexpected query: %s", req.URL.RawQuery)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"hello":"world"}`)),
			Header:     make(http.Header),
		}, nil
	})

	params := &url.Values{}
	params.Set("name", "world")
	resp := client.GetRequest("http://example.com", params)
	if resp.Error != nil {
		t.Fatalf("request failed: %v", resp.Error)
	}

	var payload map[string]string
	if err := resp.ParseJson(&payload); err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if payload["hello"] != "world" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestJsonRequestSetsContentType(t *testing.T) {
	client := HttpRequest{}
	client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("unexpected content-type: %s", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     make(http.Header),
		}, nil
	})

	options := map[string]string{}
	resp := client.JsonRequest(http.MethodPost, "http://example.com", strings.NewReader(`{"x":1}`), options)
	if resp.Error != nil {
		t.Fatalf("request failed: %v", resp.Error)
	}

	raw, err := resp.Raw()
	if err != nil {
		t.Fatalf("raw failed: %v", err)
	}
	if raw != "ok" {
		t.Fatalf("unexpected raw: %s", raw)
	}
}

func TestDefaultTimeoutIsSet(t *testing.T) {
	client := HttpRequest{}
	// 零值 Client 的 Timeout 应为 0
	if client.Client.Timeout != 0 {
		t.Fatalf("expected zero timeout before request, got %v", client.Client.Timeout)
	}

	// 发送请求后，timeout 应被设置为默认值
	client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     make(http.Header),
		}, nil
	})
	client.Request("GET", "http://example.com", nil)

	if client.Client.Timeout != defaultHTTPTimeout {
		t.Fatalf("expected timeout %v, got %v", defaultHTTPTimeout, client.Client.Timeout)
	}
}

func TestExplicitTimeoutPreserved(t *testing.T) {
	client := HttpRequest{}
	client.Client.Timeout = 5 * time.Second

	client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     make(http.Header),
		}, nil
	})
	client.Request("GET", "http://example.com", nil)

	if client.Client.Timeout != 5*time.Second {
		t.Fatalf("expected timeout 5s, got %v", client.Client.Timeout)
	}
}

func TestLargeBodyRejected(t *testing.T) {
	// 创建一个返回超大响应的测试服务器
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 写入超过 10MB 的数据
		big := make([]byte, defaultMaxBodySize+1024)
		w.Write(big)
	}))
	defer srv.Close()

	client := HttpRequest{}
	resp := client.GetRequest(srv.URL, nil)
	if resp.Error != nil {
		t.Fatalf("request failed: %v", resp.Error)
	}

	_, err := resp.ParseBytes()
	if err == nil {
		t.Fatal("expected error for large body, got nil")
	}
	if err.Error() != "http response body too large" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExactMaxBodySizeAccepted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, defaultMaxBodySize))
	}))
	defer srv.Close()

	client := HttpRequest{}
	resp := client.GetRequest(srv.URL, nil)
	if resp.Error != nil {
		t.Fatalf("request failed: %v", resp.Error)
	}

	body, err := resp.ParseBytes()
	if err != nil {
		t.Fatalf("expected exact max body size to be accepted, got %v", err)
	}
	if len(body) != defaultMaxBodySize {
		t.Fatalf("unexpected body size: got=%d want=%d", len(body), defaultMaxBodySize)
	}
}

func TestBodyJustOverMaxSizeRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, defaultMaxBodySize+1))
	}))
	defer srv.Close()

	client := HttpRequest{}
	resp := client.GetRequest(srv.URL, nil)
	if resp.Error != nil {
		t.Fatalf("request failed: %v", resp.Error)
	}

	_, err := resp.ParseBytes()
	if err == nil || err.Error() != "http response body too large" {
		t.Fatalf("expected body too large error, got %v", err)
	}
}

func TestNormalBodyAccepted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("small response"))
	}))
	defer srv.Close()

	client := HttpRequest{}
	resp := client.GetRequest(srv.URL, nil)
	if resp.Error != nil {
		t.Fatalf("request failed: %v", resp.Error)
	}

	body, err := resp.ParseBytes()
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if string(body) != "small response" {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestNewRequestErrorDoesNotPanic(t *testing.T) {
	client := HttpRequest{}
	// 传入无效字符触发 NewRequest 错误
	resp := client.Request("GET", "http://example.com\x00path", nil)
	if resp.Error == nil {
		t.Fatal("expected error for invalid URL")
	}
	// 不 panic 即为通过
}
