package functions

import "strings"

// ParseFunctionsStr 解析字符串, 返回函数名和参数列表
func ParseFunctionsStr(funcStr string) (functionsName string, paramKeys []string) {
	if !strings.Contains(funcStr, "(") { // 无参形式
		functionsName = funcStr
		return
	}

	leftIndex := strings.Index(funcStr, "(")
	functionsName = funcStr[0:leftIndex]

	paramsStr := strings.TrimSpace(funcStr[leftIndex+1 : len(funcStr)-1]) // str must endsWith )
	if paramsStr == "" {
		return
	}

	paramKeys = append(paramKeys, strings.Split(paramsStr, ",")...)

	return
}
