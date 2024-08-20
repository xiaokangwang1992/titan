/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2024/08/08 19:30:30
 Desc     :
*/

package storage

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/piaobeizu/titan/utils/cipher"
)

type Oss struct {
	provider string
	url      string
}

func NewOss(provider, url string) *Oss {
	return &Oss{
		provider: provider,
		url:      url,
	}
}

type OssLinkInfo struct {
	accessKey string
	secretKey string
	region    string
	endpoint  string
}

var CIPHER_KEY = "uOvKLmVfztaXGpNYd4Z0I1SiT7MweJhl"

func parseOssLink(url string) *OssLinkInfo {
	return nil
}

func (o *Oss) newSession() (*session.Session, error) {
	oli := parseOssLink(o.url)
	if oli == nil {
		return nil, fmt.Errorf("invalid oss link")
	}

	accessKey, err := cipher.Decrypt(oli.accessKey, CIPHER_KEY)
	if err != nil {
		return nil, err
	}
	secretKey, err := cipher.Decrypt(oli.secretKey, CIPHER_KEY)
	if err != nil {
		return nil, err
	}
	creds := credentials.NewStaticCredentials(string(accessKey), string(secretKey), "")
	region := oli.region
	endpoint := oli.endpoint
	config := &aws.Config{
		Region:           aws.String(region),
		Endpoint:         &endpoint,
		S3ForcePathStyle: aws.Bool(false),
		Credentials:      creds,
	}
	return session.NewSession(config)
}

func (o *Oss) DownObject(bucket, key, localFile string) error {
	f, err := os.Create(localFile)
	if err != nil {
		return err
	}
	defer f.Close()
	sess, _ := o.newSession()
	download := s3manager.NewDownloaderWithClient(s3.New(sess), func(d *s3manager.Downloader) {
		d.PartSize = 64 * 1024 * 1024
	})
	_, err = download.Download(f, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	return nil
}

func (o *Oss) ObjectExists(bucket, key string) (bool, error) {
	sess, err := o.newSession()
	if err != nil {
		return false, err
	}
	svc := s3.New(sess)
	_, err = svc.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && (aerr.Code() == s3.ErrCodeNoSuchKey || aerr.Code() == "NotFound") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// UploadObject 上传文件到指定的存储桶
func (o *Oss) UploadObject(bucket, key, localFile string) error {
	f, err := os.Open(localFile)
	if err != nil {
		return err
	}
	defer f.Close()
	sess, err := o.newSession()
	if err != nil {
		return err
	}
	uploader := s3manager.NewUploader(sess)
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   f,
	})
	if err != nil {
		return err
	}
	return nil
}
