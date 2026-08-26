package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type checkClient struct {
	baseURL string
	client  *http.Client
	serial  int
}

func newCheckClient(address string) *checkClient {
	return &checkClient{
		baseURL: "http://" + address,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *checkClient) key(operation string) string {
	c.serial++
	return fmt.Sprintf("selfcheck-%02d-%s", c.serial, operation)
}

func (c *checkClient) request(ctx context.Context, method, path string, body any, destination any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("编码自检请求: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("创建自检请求: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("执行 %s %s: %w", method, path, err)
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, 2<<20)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		data, _ := io.ReadAll(limited)
		return fmt.Errorf("%s %s 返回 %d: %s", method, path, response.StatusCode, string(data))
	}
	if destination != nil {
		if err := json.NewDecoder(limited).Decode(destination); err != nil {
			return fmt.Errorf("解析 %s %s 响应: %w", method, path, err)
		}
	}
	return nil
}
