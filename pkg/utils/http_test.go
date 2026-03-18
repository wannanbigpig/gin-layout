package utils

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
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
