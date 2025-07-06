/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2024/01/21 10:39:11
 Desc     :
*/

package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v2"
)

func Struct2Json(data any) string {
	str, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		panic(err)
	}
	var content = string(str)
	content = strings.Replace(content, "\\u003c", "<", -1)
	content = strings.Replace(content, "\\u003e", ">", -1)
	content = strings.Replace(content, "\\u0026", "&", -1)
	content = strings.Replace(content, "\\\\", "", -1)
	return content
}

func Struct2Yaml(data any) string {
	str, err := yaml.Marshal(data)
	if err != nil {
		panic(err)
	}
	var content = string(str)
	content = strings.Replace(content, "\\u003c", "<", -1)
	content = strings.Replace(content, "\\u003e", ">", -1)
	content = strings.Replace(content, "\\u0026", "&", -1)
	content = strings.Replace(content, "\\\\", "", -1)
	return content
}

func RemoveDuplicatesAndEmpty(a []string) (ret []string) {
	for i := range a {
		if (i > 0 && a[i-1] == a[i]) || len(a[i]) == 0 {
			continue
		}
		ret = append(ret, a[i])
	}
	return
}

func TimeDiff(t1, t2, layout string) (int64, error) {
	pt1, err := time.Parse(layout, t1)
	if err != nil {
		return 0, err
	}
	pt2, err := time.Parse(layout, t2)
	if err != nil {
		return 0, err
	}
	return pt1.Unix() - pt2.Unix(), nil
}

func AnyToStruct(src, dst any) (err error) {
	if src == nil || dst == nil {
		return fmt.Errorf("src or dst is nil")
	}
	var arr []byte
	if _, ok := src.(string); !ok {
		arr, err = json.Marshal(src)
		if err != nil {
			return err
		}
	} else {
		arr = []byte(src.(string))
	}
	err = json.Unmarshal(arr, &dst)
	if err != nil {
		return err
	}
	return nil
}

func RemoveRepeatedElement(arr []string) (newArr []string) {
	newArr = make([]string, 0)
	for i := 0; i < len(arr); i++ {
		repeat := false
		for j := i + 1; j < len(arr); j++ {
			if arr[i] == arr[j] {
				repeat = true
				break
			}
		}
		if !repeat {
			newArr = append(newArr, arr[i])
		}
	}
	return
}

// toYaml 将数据转换为 YAML 格式
func toYaml(v any) (string, error) {
	data, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func nindent(indent int, s string) string {
	pad := fmt.Sprintf("%*s", indent, "")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = pad + line
	}
	return "\n" + strings.Join(lines, "\n")
}
func NewTemplate(id, content string, data any) (string, error) {
	funcs := template.FuncMap{
		"join":    strings.Join,
		"toYaml":  toYaml,
		"nindent": nindent,
	}
	t := template.New(id).Funcs(funcs)
	t = template.Must(t.Parse(content))
	var b = new(bytes.Buffer)
	if err := t.Execute(b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

func FindCaller(skip int) (string, int) {
	file := ""
	line := 0
	for i := 0; i < 10; i++ {
		file, line = getCaller(skip + i)
		if !strings.HasPrefix(file, "logrus") {
			break
		}
	}
	return file, line
	// return fmt.Sprintf("%s:%d", file, line)
}

func getCaller(skip int) (string, int) {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "", 0
	}
	n := 0
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			n++
			if n >= 2 {
				file = file[i+1:]
				break
			}
		}
	}
	return file, line
}

func GetEnv(key, value string) string {
	if env := os.Getenv(key); env != "" {
		return env
	}
	return value
}

// inferType 自动推断字符串的数据类型
func inferType(s string) any {
	// 尝试解析为整数
	if intVal, err := strconv.Atoi(s); err == nil {
		return intVal
	}

	// 尝试解析为浮点数
	if floatVal, err := strconv.ParseFloat(s, 64); err == nil {
		return floatVal
	}

	// 尝试解析为布尔值
	if boolVal, err := strconv.ParseBool(s); err == nil {
		return boolVal
	}

	// 如果都无法解析，返回原始字符串
	return s
}

// ExtractByRegex 提取正则表达式匹配的结果，自动推断数据类型
func ExtractByRegex(pattern, text string) (map[string]any, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compile regex failed: %v", err)
	}

	match := re.FindStringSubmatch(text)
	if match == nil {
		return nil, nil
	}

	result := make(map[string]any)
	groupNames := re.SubexpNames()

	for i, name := range groupNames {
		if i != 0 {
			var key string
			if name != "" {
				key = name
			} else {
				key = fmt.Sprintf("group%d", i)
			}
			// 使用类型推断函数
			result[key] = inferType(match[i])
		}
	}

	return result, nil
}

// ExtractByRegexString 提取正则表达式匹配的结果，保持字符串类型（向后兼容）
func ExtractByRegexString(pattern, text string) (map[string]string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compile regex failed: %v", err)
	}

	match := re.FindStringSubmatch(text)
	if match == nil {
		return nil, nil
	}

	result := make(map[string]string)
	groupNames := re.SubexpNames()

	for i, name := range groupNames {
		if i != 0 {
			if name != "" {
				result[name] = match[i]
			} else {
				result[fmt.Sprintf("group%d", i)] = match[i]
			}
		}
	}

	return result, nil
}

func EqualURL(url1, url2 string, keyParams []string) (bool, error) {
	u1, err := url.Parse(url1)
	if err != nil {
		return false, err
	}
	u2, err := url.Parse(url2)
	if err != nil {
		return false, err
	}

	if u1.Scheme != u2.Scheme || u1.Host != u2.Host || u1.Path != u2.Path {
		return false, fmt.Errorf("scheme or host or path not equal: %s, %s", url1, url2)
	}

	for _, param := range keyParams {
		if u1.Query().Get(param) != u2.Query().Get(param) {
			return false, fmt.Errorf("param %s not equal: %s, %s", param, u1.Query().Get(param), u2.Query().Get(param))
		}
	}

	return true, nil

}
