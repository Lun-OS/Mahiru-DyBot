package onebot

import "fmt"

// fmtSscan 简单封装，避免直接引入 fmt 到 types.go 顶部造成循环依赖困扰。
func fmtSscan(s string, v *int64) (int, error) {
	return fmt.Sscanf(s, "%d", v)
}
