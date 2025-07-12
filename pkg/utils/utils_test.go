/*
 * @Version : 1.0
 * @Author  : wangxk
 * @Email   : wangxk1991@gmail.com
 * @Date    : 2025/07/02
 * @Desc    : 测试文件
 */

package utils

import (
	"fmt"
	"testing"
)

func TestExtractByRegex(t *testing.T) {
	pattern := `^bytes (?P<start>\d+)-(?P<end>\d+)/(?P<total>\d+)$`
	text := "bytes 0-1023/1024"
	result, err := ExtractByRegex(pattern, text)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Result:", result)
}
