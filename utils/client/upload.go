/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2025/07/02 17:43:01
 Desc     :
*/

package client

import (
	"context"
	"net/http"
	"os"

	"github.com/piaobeizu/titan/utils"
)

type UploadConfig struct {
	ChunkSize         int64
	GenerateUploadUrl string
}

func DefaultUploadConfig() *UploadConfig {
	return &UploadConfig{
		ChunkSize:         1024 * 1024 * 10,
		GenerateUploadUrl: utils.GetEnv("FILE_UPLOAD_URL", "/api/v1/upload"),
	}
}

type UploadClient struct {
	ctx     context.Context
	config  *UploadConfig
	baseUrl string
	headers map[string]string
	client  *http.Client
}

var (
	uploadUrls    = make(map[string]string)
	uploadedBytes = make(map[string]int64)
)

func NewUploadClient(ctx context.Context, baseUrl string, config *UploadConfig) *UploadClient {
	return &UploadClient{
		ctx:     ctx,
		baseUrl: baseUrl,
		config:  config,
		headers: make(map[string]string),
		client:  &http.Client{},
	}
}

func (c *UploadClient) SetHeader(key, value string) {
	c.headers[key] = value
}

func (c *UploadClient) SetHeaders(headers map[string]string) {
	for k, v := range headers {
		c.SetHeader(k, v)
	}
}

func (c *UploadClient) GetHeader(key string) string {
	return c.headers[key]
}

func (c *UploadClient) GetAllHeaders() map[string]string {
	return c.headers
}

func (c *UploadClient) RemoveHeader(key string) {
	delete(c.headers, key)
}

func (c *UploadClient) ClearHeaders() {
	c.headers = make(map[string]string)
}

func (c *UploadClient) Upload(path, filename string, mode os.FileMode) (int64, error) {

	return 0, nil
}
