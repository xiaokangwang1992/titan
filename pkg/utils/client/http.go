/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2025/06/23 16:47:00
 Desc     :
*/

package client

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

type HttpClientConfig struct {
	Timeout     int
	MaxRetries  int
	MaxBodySize int64
}

func DefaultHttpClientConfig() HttpClientConfig {
	return HttpClientConfig{
		Timeout:     3,
		MaxRetries:  3,
		MaxBodySize: 1024 * 1024 * 1024 * 100, // 100GB
	}
}

type HttpClient struct {
	client *http.Client
	config HttpClientConfig
}

func NewHttpClient(config HttpClientConfig) *HttpClient {
	return &HttpClient{
		client: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
		config: config,
	}
}

func (c *HttpClient) Get(url string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *HttpClient) Post(url string, headers map[string]string, body any) (*http.Response, error) {
	json, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(json))
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
