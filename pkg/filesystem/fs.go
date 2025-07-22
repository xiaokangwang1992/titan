/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2025/07/01 19:26:58
 Desc     : file system service
*/

package filesystem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	nurl "net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/piaobeizu/titan/config"
	"github.com/piaobeizu/titan/pkg/utils"
	"github.com/piaobeizu/titan/pkg/utils/cipher"
	"github.com/sirupsen/logrus"
)

var (
	ErrFileAlreadyExists    = errors.New("file already exists")
	ErrFileUploading        = errors.New("file is uploading")
	ErrUploadURLExpired     = errors.New("upload url expired")
	ErrMetaFileNotFound     = errors.New("meta file not found")
	ErrFileUploadIncomplete = errors.New("file upload incomplete")
)

func DefaultFileSystemConfig() *config.FileSystem {
	return &config.FileSystem{
		FileUploader: &config.FileUploader{
			FileMaxSize: 1024 * 1024 * 1024 * 100, // 100GB
			ChunkSize:   1024 * 1024 * 100,        // 100MB
			BufferSize:  1024 * 1024,              // 1MB
			FileTypes: []string{"application/octet-stream", "image/jpeg", "image/png", "image/gif", "image/webp", "application/pdf",
				"application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/vnd.ms-excel",
				"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "application/vnd.ms-powerpoint",
				"application/vnd.openxmlformats-officedocument.presentationml.presentation", "application/zip", "application/x-7z-compressed",
				"application/x-rar-compressed", "application/x-tar", "application/x-gzip", "application/x-bzip2", "application/x-xz",
				"application/x-zstd", "application/x-lzip", "application/x-lzma", "application/x-lz4", "application/x-zstd",
				"application/x-lzip", "application/x-lzma", "application/x-lz4", "application/x-zstd", "application/x-lzip",
				"application/x-lzma", "application/x-lz4", "application/x-zstd", "application/x-lzip", "application/x-lzma",
				"application/x-lz4", "application/x-zstd", "application/x-lzip", "application/x-lzma", "application/x-lz4",
			},
			FormName:   "file",
			ExpireTime: 3600, // 1 hour
			PathMode:   0755, // 0755
		},
	}
}

type FileMeta struct {
	MD5     string `json:"md5"`
	Status  string `json:"status"` // uploading, completed, incomplete, unknown
	Expired int64  `json:"expired"`
	Type    string `json:"type"`
}

type FileSystem struct {
	ctx     context.Context
	config  *config.FileSystem
	logger  *logrus.Entry
	baseDir string
}

type PathParams struct {
	Show  bool
	Key   string
	Value string
}

func NewFileSystem(ctx context.Context, baseDir string, config *config.FileSystem) *FileSystem {
	return &FileSystem{
		ctx:     ctx,
		config:  config,
		baseDir: baseDir,
		logger:  logrus.WithField("module", "filesystem"),
	}
}

func (u *FileSystem) GenerateUploadURL(url, path, filename, secret string, pathParams []PathParams) (string, error) {
	var (
		meta       *FileMeta
		err        error
		absPath    = u.getAbsPath(path)
		showParams = []string{}
	)
	for _, param := range pathParams {
		showParams = append(showParams, fmt.Sprintf("%s=%s", param.Key, param.Value))
	}
	showParams = append(showParams, []string{
		fmt.Sprintf("file=%s", filename),
		fmt.Sprintf("path=%s", path),
	}...)
	secret, err = cipher.EncryptCompact([]byte(strings.Join(showParams, "&")), secret)
	if err != nil {
		return "", err
	}

	showParams = []string{fmt.Sprintf("secret=%s", secret), fmt.Sprintf("file=%s", filename), fmt.Sprintf("path=%s", path)}
	for _, param := range pathParams {
		if param.Show {
			showParams = append(showParams, fmt.Sprintf("%s=%s", param.Key, param.Value))
		}
	}
	u.logger.Infof("generate upload url: %s?%s", url, strings.Join(showParams, "&"))

	if meta, err = u.getFileMeta(absPath, filename); err == nil {
		meta.Expired = time.Now().Unix() + u.config.FileUploader.ExpireTime
	} else {
		meta = &FileMeta{
			MD5:     "",
			Status:  "incomplete",
			Expired: time.Now().Unix() + u.config.FileUploader.ExpireTime,
		}
	}
	u.logger.Infof("generate meta: %v", meta)
	if err := u.setFileMeta(absPath, filename, meta); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s?%s", url, strings.Join(showParams, "&")), nil
}

func (u *FileSystem) CheckUrl(urlObj *nurl.URL, secret string, keyParams []string) error {
	if urlObj == nil {
		return fmt.Errorf("url is nil")
	}
	usecret := urlObj.Query().Get("secret")

	dsecret, err := cipher.DecryptCompact(usecret, secret)
	if err != nil {
		return fmt.Errorf("decrypt secret failed: %v", err)
	}

	ur, err := nurl.Parse("?" + string(dsecret))
	if err != nil {
		return fmt.Errorf("parse url failed: %v", err)
	}

	if ok, err := utils.EqualURL(ur.String(), urlObj.String(), keyParams); !ok {
		return fmt.Errorf("invalid url: %s, please check your url", err.Error())
	}

	return nil
}

func (u *FileSystem) UploadFile(c *gin.Context, path, filename string, overwrite bool, mode os.FileMode) (int64, error) {
	var (
		meta    *FileMeta
		err     error
		absPath = u.getAbsPath(path)
	)
	defer func() {
		if err := u.setFileMeta(absPath, filename, meta); err != nil {
			u.logger.Errorf("set file meta failed: %s", err)
		}
	}()
	u.logger.Infof("upload file: %s, %s, %d", path, filename, mode)
	// TODO: 这个地方需要优化，文件锁在 getFileMeta 中获取，但是 setFileMeta 中没有释放，需要优化
	if meta, err = u.getFileMeta(absPath, filename); err != nil {
		return 0, ErrMetaFileNotFound
	}
	if overwrite {
		os.Remove(filepath.Join(absPath, filename))
		meta.Status = "incomplete"
		meta.MD5 = ""
	}
	if meta.Status == "uploading" {
		return 0, ErrFileUploading
	}
	if meta.Status == "completed" {
		return 0, ErrFileAlreadyExists
	}
	if time.Now().Unix() > meta.Expired {
		return 0, ErrUploadURLExpired
	}

	// check content type
	contentType := c.GetHeader("Content-Type")
	support := false
	for _, fileType := range u.config.FileUploader.FileTypes {
		if strings.HasPrefix(contentType, fileType) {
			support = true
			break
		}
	}
	if !support {
		return 0, fmt.Errorf("unsupported Content-Type: %s, we only support %s", contentType, strings.Join(u.config.FileUploader.FileTypes, ", "))
	}
	meta.Type = contentType

	meta.Status = "uploading"
	if err := u.setFileMeta(absPath, filename, meta); err != nil {
		return 0, err
	}

	// check content range
	contentRange := c.GetHeader("Content-Range")
	contentRangeMap, err := utils.ExtractByRegex(`^bytes (?P<start>\d+)-(?P<end>\d+)/(?P<total>\d+)$`, contentRange)
	if err != nil || contentRangeMap == nil {
		return 0, fmt.Errorf("extract content range failed: %v", err)
	}
	u.logger.Infof("content range: %v", contentRangeMap)
	totalSize := int64(contentRangeMap["total"].(int))
	if totalSize > u.config.FileUploader.FileMaxSize {
		return 0, fmt.Errorf("file size exceeds the maximum limit: %d > %d", totalSize, u.config.FileUploader.FileMaxSize)
	}
	start := int64(contentRangeMap["start"].(int))

	if mode == 0 {
		mode = u.config.FileUploader.PathMode // Default file permissions
	}
	// upload file
	md5, uploadSize, err := u.uploadOSFile(c, absPath, filename, mode, start, totalSize)
	meta.MD5 = md5
	meta.Status = "incomplete"
	if err == nil {
		meta.Status = "completed"
	}
	return uploadSize, err
}

func (u *FileSystem) ListDir(path string, hidden bool) (items []map[string]any, err error) {
	var (
		absPath = u.getAbsPath(path)
		info    os.FileInfo
	)

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return
	}

	info, err = os.Stat(absPath)
	if err != nil {
		return
	}
	items = append(items, map[string]any{
		"path":     "/",
		"size":     info.Size(),
		"mode":     info.Mode(),
		"mod_time": info.ModTime().Format(time.DateTime),
		"is_dir":   true,
		"md5":      "",
	})
	for _, entry := range entries {
		// ignore hidden file
		if !hidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if hidden && strings.HasSuffix(entry.Name(), ".meta") {
			continue
		}
		info, err = entry.Info()
		if err != nil {
			return
		}
		md5 := ""
		if !info.IsDir() {
			md5, err = u.MD5(path, entry.Name())
			if err != nil {
				return
			}
		}
		items = append(items, map[string]any{
			"path":     strings.TrimPrefix(entry.Name(), u.baseDir),
			"size":     info.Size(),
			"mode":     info.Mode(),
			"mod_time": info.ModTime().Format(time.DateTime),
			"is_dir":   entry.IsDir(),
			"md5":      md5,
		})
	}

	return
}

func (u *FileSystem) CreateDir(path string, mode os.FileMode) error {
	absPath := u.getAbsPath(path)
	if mode == 0 {
		mode = u.config.FileUploader.PathMode
	}
	if err := os.MkdirAll(absPath, mode); err != nil {
		return err
	}
	return os.Chmod(absPath, mode)
}

func (u *FileSystem) AddFileTypes(types []string) {
	if len(types) == 0 || u.config.FileUploader == nil {
		return
	}
	if u.config.FileUploader.FileTypes == nil {
		u.config.FileUploader.FileTypes = []string{}
	}
	u.config.FileUploader.FileTypes = append(u.config.FileUploader.FileTypes, types...)
	u.config.FileUploader.FileTypes = utils.RemoveDuplicatesAndEmpty(u.config.FileUploader.FileTypes)
}

func (u *FileSystem) DeletePath(path string) error {
	absPath := u.getAbsPath(path)
	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		metaFile := u.getMetaPath(filepath.Dir(absPath), info.Name())
		if err := os.Remove(metaFile); err != nil {
			return err
		}
		return os.Remove(absPath)
	}
	return os.RemoveAll(absPath)
}

func (u *FileSystem) MD5(path, filename string) (string, error) {
	absPath := u.getAbsPath(path)
	meta, err := u.getFileMeta(absPath, filename)
	if err != nil {
		return "", err
	}
	return meta.MD5, nil
}

func (u *FileSystem) uploadOSFile(c *gin.Context, absPath, filename string, mode os.FileMode, start, totalSize int64) (string, int64, error) {
	tempFilePath := utils.GetTempFilePath(absPath, filename)
	info, _ := os.Stat(tempFilePath)
	if info != nil {
		if info.Size() > totalSize {
			return "", info.Size(), fmt.Errorf("file size is greater than the total size: %d > %d", info.Size(), totalSize)
		}
		if info.Size() > start {
			return "md5", info.Size(), ErrFileUploadIncomplete
		}
	}
	tmpFile, err := os.OpenFile(tempFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, mode)
	if err != nil {
		return "", 0, fmt.Errorf("open file failed: %v", err)
	}
	defer tmpFile.Close()

	var (
		uploadSize int64
		buffer     = make([]byte, u.config.FileUploader.BufferSize)
		n          int
		uploadMB   int64
		md5        string
	)
mainloop:
	for {
		select {
		case <-u.ctx.Done():
			u.logger.Infof("❌ server context canceled, upload interrupted")
			err = fmt.Errorf("server context canceled")
			break mainloop
		case <-c.Request.Context().Done():
			u.logger.Infof("❌ client context canceled, connection interrupted")
			err = fmt.Errorf("client connection interrupted")
			break mainloop
		default:
			n, err = c.Request.Body.Read(buffer)
			if n > 0 {
				written, writeErr := tmpFile.Write(buffer[:n])
				if writeErr != nil {
					err = fmt.Errorf("write file failed: %v", writeErr)
					break mainloop
				}
				uploadSize += int64(written)

				if uploadSize/(1024*1024) != uploadMB { // 每1MB记录一次
					uploadMB = uploadSize / (1024 * 1024)
					u.logger.Infof("📊 file %s upload progress: %d MB", filename, uploadMB)
				}
			}
			if err == io.EOF {
				break mainloop
			}
			if err != nil {
				err = fmt.Errorf("read file failed: %v", err)
				break mainloop
			}
		}
	}
	if err != nil && err != io.EOF {
		return md5, 0, err
	}
	info, err = os.Stat(tempFilePath)
	if err != nil {
		return md5, info.Size(), err
	}
	u.logger.Infof("file %s upload size: %d, total size: %d", filename, info.Size(), totalSize)
	if info.Size() < totalSize {
		return md5, info.Size(), ErrFileUploadIncomplete
	}
	os.Rename(tempFilePath, filepath.Join(absPath, filename))
	md5, err = utils.CalFileMD5(filepath.Join(absPath, filename))
	return md5, info.Size(), err
}

func (u *FileSystem) getFileMeta(absPath, fileName string) (*FileMeta, error) {
	metaFile := u.getMetaPath(absPath, fileName)
	var meta FileMeta
	if err := utils.ReadFileToStruct(metaFile, &meta, "json", true); err != nil {
		return nil, err
	}
	return &meta, nil
}

func (u *FileSystem) setFileMeta(absPath, fileName string, meta *FileMeta) error {
	metaFile := u.getMetaPath(absPath, fileName)
	if meta == nil {
		meta = &FileMeta{
			MD5:     "",
			Status:  "unknown",
			Expired: 0,
		}
	}
	metaStr, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	u.logger.Infof("set file meta: %s, %v", metaFile, string(metaStr))
	if err := utils.WriteFile(metaFile, metaStr, 0644, true); err != nil {
		return err
	}
	return nil
}

func (u *FileSystem) getMetaPath(absPath, fileName string) string {
	fileName = strings.TrimPrefix(fileName, "/")
	fileName = strings.TrimPrefix(fileName, ".")
	fileName = strings.TrimSuffix(fileName, ".tmp")
	return filepath.Join(absPath, fmt.Sprintf(".%s.meta", fileName))
}

func (u *FileSystem) getAbsPath(path string) string {
	path = strings.TrimPrefix(path, "/")
	// path = strings.TrimPrefix(path, ".")
	return filepath.Join(u.baseDir, path)
}

// GetFileInfo validates and returns file information for download
func (u *FileSystem) GetFileInfo(path, filename string) (string, os.FileInfo, error) {
	absPath := u.getAbsPath(path)
	fullPath := filepath.Join(absPath, filename)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return "", nil, fmt.Errorf("file %s not found", filename)
	}

	stat, err := os.Stat(fullPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get file info: %v", err)
	}

	if stat.IsDir() {
		return "", nil, fmt.Errorf("path is a directory, not a file")
	}

	return fullPath, stat, nil
}

// GetPathSize calculates the total size, file count, and directory count of a path
func (u *FileSystem) GetPathSize(path string) (int64, int64, int64, error) {
	absPath := u.getAbsPath(path)

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return 0, 0, 0, fmt.Errorf("path %s not found", path)
	}

	return u.calculateDirectorySize(absPath)
}

// calculateDirectorySize calculate directory size recursively
func (u *FileSystem) calculateDirectorySize(dirPath string) (int64, int64, int64, error) {
	var totalSize int64
	var fileCount int64
	var dirCount int64

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			dirCount++
		} else {
			fileCount++
			totalSize += info.Size()
		}

		return nil
	})

	return totalSize, fileCount, dirCount, err
}

// FormatBytes format bytes to human readable format
func (u *FileSystem) FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// DownloadFile downloads a file from the filesystem with metadata handling
func (u *FileSystem) DownloadFile(c *gin.Context, path, filename string) (int64, error) {
	var (
		meta    *FileMeta
		err     error
		absPath = u.getAbsPath(path)
	)
	defer func() {
		if err := u.setFileMeta(absPath, filename, meta); err != nil {
			u.logger.Errorf("set file meta failed: %s", err)
		}
	}()
	u.logger.Infof("download file: %s, %s", path, filename)

	// get file meta
	if meta, err = u.getFileMeta(absPath, filename); err != nil {
		u.logger.Warnf("meta file not found for %s, creating new meta", filename)
		meta = &FileMeta{
			MD5:     "",
			Status:  "unknown",
			Expired: 0,
			Type:    "",
		}
	}

	// check if file exists
	fullPath := filepath.Join(absPath, filename)
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("file %s not found", filename)
		}
		return 0, fmt.Errorf("failed to get file info: %v", err)
	}

	// check if it is a directory
	if info.IsDir() {
		return 0, fmt.Errorf("path is a directory, not a file")
	}

	// update meta
	meta.Status = "downloading"
	meta.Expired = time.Now().Unix() + 3600 // 1 hour expired
	if err := u.setFileMeta(absPath, filename, meta); err != nil {
		return 0, fmt.Errorf("failed to update file meta: %v", err)
	}

	// check Range header (support range download)
	rangeHeader := c.GetHeader("Range")
	var start int64 = 0
	var end int64 = info.Size() - 1

	if rangeHeader != "" {
		rangeMap, err := utils.ExtractByRegex(`^bytes=(?P<start>\d+)-(?P<end>\d*)$`, rangeHeader)
		if err != nil || rangeMap == nil {
			return 0, fmt.Errorf("invalid range header: %s", rangeHeader)
		}

		startVal := rangeMap["start"]
		endVal := rangeMap["end"]

		// handle start value
		switch v := startVal.(type) {
		case int:
			start = int64(v)
		case string:
			start, err = strconv.ParseInt(v, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid start range: %s", v)
			}
		default:
			return 0, fmt.Errorf("invalid start range type: %T", startVal)
		}

		// handle end value
		if endVal != nil {
			switch v := endVal.(type) {
			case int:
				end = int64(v)
			case string:
				if v != "" {
					end, err = strconv.ParseInt(v, 10, 64)
					if err != nil {
						return 0, fmt.Errorf("invalid end range: %s", v)
					}
				}
			default:
				return 0, fmt.Errorf("invalid end range type: %T", endVal)
			}
		}

		// validate range
		if start < 0 || start >= info.Size() || end >= info.Size() || start > end {
			return 0, fmt.Errorf("range out of bounds: %d-%d, file size: %d", start, end, info.Size())
		}
	}

	// set response headers
	contentLength := end - start + 1
	c.Header("Content-Length", fmt.Sprintf("%d", contentLength))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Accept-Ranges", "bytes")

	if rangeHeader != "" {
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, info.Size()))
		c.Status(http.StatusPartialContent)
	} else {
		c.Status(http.StatusOK)
	}

	// execute download
	downloadSize, err := u.downloadOSFile(c, absPath, filename, start, end)
	if err != nil {
		meta.Status = "download_failed"
		return downloadSize, err
	}

	// update meta
	meta.Status = "completed"
	meta.Expired = time.Now().Unix() + 3600 // 1 hour expired

	u.logger.Infof("file %s download completed: %d bytes", filename, downloadSize)
	return downloadSize, nil
}

// downloadOSFile downloads a file with range support
func (u *FileSystem) downloadOSFile(c *gin.Context, absPath, filename string, start, end int64) (int64, error) {
	fullPath := filepath.Join(absPath, filename)

	// open file
	file, err := os.Open(fullPath)
	if err != nil {
		return 0, fmt.Errorf("open file failed: %v", err)
	}
	defer file.Close()

	// if start is specified, jump to the specified position
	if start > 0 {
		_, err = file.Seek(start, io.SeekStart)
		if err != nil {
			return 0, fmt.Errorf("seek to position %d failed: %v", start, err)
		}
	}

	var (
		downloadSize int64
		buffer       = make([]byte, u.config.FileUploader.BufferSize)
		n            int
		downloadMB   int64
		remaining    = end - start + 1
	)

mainloop:
	for {
		select {
		case <-u.ctx.Done():
			u.logger.Infof("❌ server context canceled, download interrupted")
			return downloadSize, fmt.Errorf("server context canceled")
		case <-c.Request.Context().Done():
			u.logger.Infof("❌ client context canceled, connection interrupted")
			return downloadSize, fmt.Errorf("client connection interrupted")
		default:
			// calculate the size of the current read
			readSize := int64(len(buffer))
			if remaining < readSize {
				readSize = remaining
			}

			if readSize <= 0 {
				break mainloop
			}

			n, err = file.Read(buffer[:readSize])
			if n > 0 {
				written, writeErr := c.Writer.Write(buffer[:n])
				if writeErr != nil {
					return downloadSize, fmt.Errorf("write to response failed: %v", writeErr)
				}
				downloadSize += int64(written)
				remaining -= int64(written)

				// record download progress every 1MB
				if downloadSize/(1024*1024) != downloadMB {
					downloadMB = downloadSize / (1024 * 1024)
					u.logger.Infof("📊 file %s download progress: %d MB", filename, downloadMB)
				}
			}
			if err == io.EOF {
				break mainloop
			}
			if err != nil {
				return downloadSize, fmt.Errorf("read file failed: %v", err)
			}
		}
	}

	u.logger.Infof("file %s range download completed: %d bytes (%d-%d)", filename, downloadSize, start, end)
	return downloadSize, nil
}
