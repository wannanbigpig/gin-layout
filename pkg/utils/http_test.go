package utils

import (
	"fmt"
	"testing"
)

func TestGetRequest(t *testing.T) {
	http := HttpRequest{}
	_, err := http.Request("GET", "https://www.baidu.com", nil).ParseBytes()
	if err != nil {
		t.Error("请求失败")
	}

}

func ExampleRequest() {
	http := HttpRequest{}
	// You can define map directly or define a structure corresponding to the return value to receive data
	var resp map[string]any
	err := http.Request("GET", "http://127.0.0.1:9999/api/v1/hello-world?name=world", nil).ParseJson(&resp)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%#v", resp)
}
