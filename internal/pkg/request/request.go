package request

import (
	"bytes"
	"errors"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
)

func GetQueryParams(c *gin.Context) map[string]any {
	query := c.Request.URL.Query()
	var queryMap = make(map[string]any, len(query))
	for k := range query {
		queryMap[k] = c.Query(k)
	}
	return queryMap
}

func GetPostFormParams(c *gin.Context) (map[string]any, error) {
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		if !errors.Is(err, http.ErrNotMultipart) {
			return nil, err
		}
	}
	var postMap = make(map[string]any, len(c.Request.PostForm))
	for k, v := range c.Request.PostForm {
		if len(v) > 1 {
			postMap[k] = v
		} else if len(v) == 1 {
			postMap[k] = v[0]
		}
	}

	return postMap, nil
}

func GetBody(c *gin.Context) []byte {
	// 读取body数据
	body, err := c.GetRawData()
	if err != nil {
		return nil
	}
	//把读过的字节流重新放到body
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	return body
}
