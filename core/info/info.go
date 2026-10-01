package info

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-runewidth"
)

const (
	boxWidth          = 56
	verticalBorder    = "|"
	horizontalBorder  = "─"
	leftTopBorder     = "┌"
	rightTopBorder    = "┐"
	leftBottomBorder  = "└"
	rightBottomBorder = "┘"

	// maxContentWidth 内容区域的最大显示宽度（列）
	// 盒子总宽度减去左右边框各 1 列，再减去内容前预留的 1 个空格
	maxContentWidth = boxWidth - 3
)

// widthCondition 用于计算终端显示宽度
// 显式关闭东亚宽度模式，使 box-drawing 等 Ambiguous 字符按 1 列计算，
// 同时中文等 Wide/Fullwidth 字符仍按 2 列计算
var widthCondition = &runewidth.Condition{
	EastAsianWidth:     false,
	StrictEmojiNeutral: true,
}

// Print 打印分组信息到标准输出
// @param name string 分组标题
// @param rows ...string 分组内容
func Print(name string, rows ...string) {
	Fprint(os.Stdout, name, rows...)
}

// Fprint 将分组信息写入指定 writer
// @param w io.Writer 输出目标
// @param name string 分组标题
// @param rows ...string 分组内容
func Fprint(w io.Writer, name string, rows ...string) {
	builder := &strings.Builder{}
	builder.WriteString(buildTopBorder(name))
	builder.WriteString("\n")
	for _, row := range rows {
		builder.WriteString(buildRowInfo(row))
		builder.WriteString("\n")
	}
	builder.WriteString(buildBottomBorder())
	builder.WriteString("\n")

	fmt.Fprint(w, builder.String())
}

// HorizontalLine 返回一条用于填充行的水平分隔线
func HorizontalLine() string {
	return strings.Repeat(horizontalBorder, maxContentWidth)
}

func buildRowInfo(info string) string {
	info = widthCondition.Truncate(info, maxContentWidth, "…")

	str := fmt.Sprintf("%s %s", verticalBorder, info)
	padding := max(0, boxWidth-widthCondition.StringWidth(str)-1)
	str += strings.Repeat(" ", padding)
	str += verticalBorder
	return str
}

func buildTopBorder(name ...string) string {
	var nameStr string
	if len(name) > 0 {
		nameStr = widthCondition.Truncate(name[0], maxContentWidth, "…")
	}

	full := max(0, boxWidth-2-widthCondition.StringWidth(nameStr))
	half := full / 2

	builder := &strings.Builder{}
	builder.WriteString(leftTopBorder)
	builder.WriteString(strings.Repeat(horizontalBorder, half))
	builder.WriteString(nameStr)
	builder.WriteString(strings.Repeat(horizontalBorder, full-half))
	builder.WriteString(rightTopBorder)
	return builder.String()
}

func buildBottomBorder() string {
	builder := &strings.Builder{}
	builder.WriteString(leftBottomBorder)
	builder.WriteString(strings.Repeat(horizontalBorder, boxWidth-2))
	builder.WriteString(rightBottomBorder)
	return builder.String()
}
