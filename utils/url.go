/*
 * @Version : 1.0
 * @Author  : xiaokang.w
 * @Email   : xiaokang.w@gmicloud.ai
 * @Date    : 2025/06/29
 * @Desc    : 描述信息
 */

package utils

import (
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
)

// URL比较相关函数
// 基本URL比较：严格比较所有组件
func EqualURL(url1, url2 string) bool {
	u1, err := url.Parse(url1)
	if err != nil {
		return false
	}
	u2, err := url.Parse(url2)
	if err != nil {
		return false
	}

	// 比较各个组件
	if u1.Scheme != u2.Scheme {
		return false
	}
	if u1.Host != u2.Host {
		return false
	}
	if u1.Path != u2.Path {
		return false
	}
	if u1.RawQuery != u2.RawQuery {
		return false
	}
	return true
}

// 参数顺序不敏感的URL比较
func EqualURLIgnoreOrder(url1, url2 string) bool {
	u1, err := url.Parse(url1)
	if err != nil {
		return false
	}
	u2, err := url.Parse(url2)
	if err != nil {
		return false
	}

	// 比较除查询参数外的其他组件
	if u1.Scheme != u2.Scheme || u1.Host != u2.Host || u1.Path != u2.Path {
		return false
	}

	// 比较查询参数（忽略顺序）
	return EqualQueryParams(u1.Query(), u2.Query())
}

// 比较查询参数（忽略顺序）
func EqualQueryParams(params1, params2 url.Values) bool {
	if len(params1) != len(params2) {
		return false
	}

	for key, values1 := range params1 {
		values2, exists := params2[key]
		if !exists {
			return false
		}

		if len(values1) != len(values2) {
			return false
		}

		// 比较值（也忽略顺序）
		valueMap := make(map[string]int)
		for _, v := range values1 {
			valueMap[v]++
		}
		for _, v := range values2 {
			valueMap[v]--
			if valueMap[v] < 0 {
				return false
			}
		}
		for _, count := range valueMap {
			if count != 0 {
				return false
			}
		}
	}

	return true
}

// 处理URL编码的参数比较（解码后比较）
func EqualQueryParamsDecoded(params1, params2 url.Values) bool {
	if len(params1) != len(params2) {
		return false
	}

	// 创建解码后的参数映射
	decoded1 := make(map[string][]string)
	decoded2 := make(map[string][]string)

	// 解码第一个参数集
	for key, values := range params1 {
		decodedKey, _ := url.QueryUnescape(key)
		var decodedValues []string
		for _, value := range values {
			decodedValue, _ := url.QueryUnescape(value)
			decodedValues = append(decodedValues, decodedValue)
		}
		decoded1[decodedKey] = decodedValues
	}

	// 解码第二个参数集
	for key, values := range params2 {
		decodedKey, _ := url.QueryUnescape(key)
		var decodedValues []string
		for _, value := range values {
			decodedValue, _ := url.QueryUnescape(value)
			decodedValues = append(decodedValues, decodedValue)
		}
		decoded2[decodedKey] = decodedValues
	}

	// 比较解码后的参数
	return EqualQueryParams(decoded1, decoded2)
}

// 智能URL比较：处理编码和参数顺序
func EqualURLSmart(url1, url2 string) bool {
	u1, err := url.Parse(url1)
	if err != nil {
		return false
	}
	u2, err := url.Parse(url2)
	if err != nil {
		return false
	}

	// 比较基本组件
	if u1.Scheme != u2.Scheme || u1.Host != u2.Host || u1.Path != u2.Path {
		return false
	}

	// 智能比较查询参数（处理编码和顺序）
	return EqualQueryParamsDecoded(u1.Query(), u2.Query())
}

// 忽略特定参数的URL比较（比如忽略文件大小参数）
func EqualURLIgnoreParams(url1, url2 string, ignoreParams []string) bool {
	u1, err := url.Parse(url1)
	if err != nil {
		return false
	}
	u2, err := url.Parse(url2)
	if err != nil {
		return false
	}

	// 比较基本组件
	if u1.Scheme != u2.Scheme || u1.Host != u2.Host || u1.Path != u2.Path {
		return false
	}

	// 创建忽略特定参数的查询参数副本
	params1 := u1.Query()
	params2 := u2.Query()

	// 移除要忽略的参数
	for _, param := range ignoreParams {
		delete(params1, param)
		delete(params2, param)
	}

	return EqualQueryParamsDecoded(params1, params2)
}

// 针对您的文件上传场景的专用比较函数
func EqualUploadURL(url1, url2 string) bool {
	// 解析URL
	u1, err := url.Parse(url1)
	if err != nil {
		return false
	}
	u2, err := url.Parse(url2)
	if err != nil {
		return false
	}

	params1 := u1.Query()
	params2 := u2.Query()

	// 提取关键参数（忽略文件大小）
	keyParams := []string{"path", "kind", "name", "is_org", "expire", "secret"}

	// 比较关键参数
	for _, param := range keyParams {
		val1 := params1.Get(param)
		val2 := params2.Get(param)
		if val1 != val2 {
			return false
		}
	}

	// 比较文件名参数（忽略文件大小值，只比较文件名）
	fileParams1 := make(map[string]bool)
	fileParams2 := make(map[string]bool)

	for key := range params1 {
		if !contains(keyParams, key) {
			// 解码文件名
			decodedKey, _ := url.QueryUnescape(key)
			fileParams1[decodedKey] = true
		}
	}

	for key := range params2 {
		if !contains(keyParams, key) {
			// 解码文件名
			decodedKey, _ := url.QueryUnescape(key)
			fileParams2[decodedKey] = true
		}
	}

	// 比较文件名集合
	if len(fileParams1) != len(fileParams2) {
		return false
	}

	for filename := range fileParams1 {
		if !fileParams2[filename] {
			return false
		}
	}

	return true
}

// 辅助函数：检查字符串是否在切片中
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func ParamInURL(urlStr, param string) bool {
	logrus.Infof("check param in url: %s, %s", urlStr, param)

	if strings.Contains(urlStr, param) {
		logrus.Infof("match param: %s", param)
		return true
	}

	encodedParam := url.QueryEscape(param)
	if strings.Contains(urlStr, encodedParam) {
		logrus.Infof("match param: %s -> %s", param, encodedParam)
		return true
	}

	if decodedURL, err := url.QueryUnescape(urlStr); err == nil {
		if strings.Contains(decodedURL, param) {
			logrus.Infof("match param: %s -> %s", urlStr, decodedURL)
			return true
		}
	}

	if parsedURL, err := url.Parse(urlStr); err == nil {
		values := parsedURL.Query()
		for key := range values {
			// 检查参数名是否匹配文件名
			if key == param {
				logrus.Infof("match param: %s", param)
				return true
			}
			// 检查解码后的参数名是否匹配
			if decodedKey, err := url.QueryUnescape(key); err == nil && decodedKey == param {
				logrus.Infof("match param: %s -> %s", key, decodedKey)
				return true
			}
		}
	}

	logrus.Warnf("param not in url: %s, %s", param, urlStr)
	return false
}

// 调试用：打印URL差异
func CompareURLDebug(url1, url2 string) {
	logrus.Infof("=== URL比较调试 ===")
	logrus.Infof("URL1: %s", url1)
	logrus.Infof("URL2: %s", url2)

	u1, err1 := url.Parse(url1)
	u2, err2 := url.Parse(url2)

	if err1 != nil {
		logrus.Errorf("URL1解析失败: %v", err1)
		return
	}
	if err2 != nil {
		logrus.Errorf("URL2解析失败: %v", err2)
		return
	}

	logrus.Infof("Scheme: %s vs %s, 相等: %t", u1.Scheme, u2.Scheme, u1.Scheme == u2.Scheme)
	logrus.Infof("Host: %s vs %s, 相等: %t", u1.Host, u2.Host, u1.Host == u2.Host)
	logrus.Infof("Path: %s vs %s, 相等: %t", u1.Path, u2.Path, u1.Path == u2.Path)
	logrus.Infof("RawQuery: %s vs %s, 相等: %t", u1.RawQuery, u2.RawQuery, u1.RawQuery == u2.RawQuery)

	params1 := u1.Query()
	params2 := u2.Query()

	logrus.Infof("参数1: %+v", params1)
	logrus.Infof("参数2: %+v", params2)

	logrus.Infof("基本比较: %t", EqualURL(url1, url2))
	logrus.Infof("忽略顺序比较: %t", EqualURLIgnoreOrder(url1, url2))
	logrus.Infof("智能比较: %t", EqualURLSmart(url1, url2))
	logrus.Infof("上传URL比较: %t", EqualUploadURL(url1, url2))
}
