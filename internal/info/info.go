package info

import (
	"fmt"
	"github.com/dobyte/due/v2/mode"
	"strings"
	"syscall"
	"unicode/utf8"
)

const logo = `
				    ____  __  ________
   			       / __ \/ / / / ____/	
  			      / / / / / / / __/
 		         / /_/ / /_/ / /___
			    /_____/\____/_____/
`

const (
	boxWidth          = 56
	topBorder         = "┌──────────────────────────────────────────────────────┐"
	bottomBorder      = "└──────────────────────────────────────────────────────┘"
	verticalBorder    = "|"
	horizontalBorder  = "─"
	leftTopBorder     = "┌"
	rightTopBorder    = "┐"
	leftBottomBorder  = "└"
	rightBottomBorder = "┘"
	website           = "https://github.com/dobyte/due"
	version           = "v2.1.0"
)

func PrintFrameworkInfo() {
	fmt.Println(strings.TrimSuffix(strings.TrimPrefix(logo, "\n"), "\n"))
	fmt.Println(topBorder)
	fmt.Println(buildRowInfo("Website", website))
	fmt.Println(buildRowInfo("Version", version))
	fmt.Println(bottomBorder)
}

func buildRowInfo(name string, value string) string {
	str := fmt.Sprintf("%s [%s] %s", verticalBorder, name, value)
	str += strings.Repeat(" ", utf8.RuneCountInString(topBorder)-utf8.RuneCountInString(str)-1)
	str += verticalBorder
	return str
}

func PrintGlobalInfo() {
	infos := make([]string, 0)
	infos = append(infos, fmt.Sprintf("PID: %d", syscall.Getpid()))
	infos = append(infos, fmt.Sprintf("Mode: %s", mode.GetMode()))

	PrintGroupInfo("Global", infos...)
}

// PrintGroupInfo 打印分组信息
func PrintGroupInfo(name string, infos ...string) {
	fmt.Println(buildTopBorder(name))
	for _, info := range infos {
		fmt.Println(buildRowsInfo(info))
	}
	fmt.Println(buildBottomBorder())
}

func buildRowsInfo(info string) string {
	str := fmt.Sprintf("%s %s", verticalBorder, info)
	str += strings.Repeat(" ", utf8.RuneCountInString(topBorder)-utf8.RuneCountInString(str)-1)
	str += verticalBorder
	return str
}

// 构建上边
func buildTopBorder(name ...string) string {
	full := boxWidth - strLen(leftTopBorder) - strLen(rightTopBorder) - strLen(name...)
	half := full / 2

	str := leftTopBorder
	str += strings.Repeat(horizontalBorder, half)
	if len(name) > 0 {
		str += name[0]
	}
	str += strings.Repeat(horizontalBorder, full-half)
	str += rightTopBorder

	return str
}

// 构建下边
func buildBottomBorder() string {
	full := boxWidth - strLen(leftBottomBorder) - strLen(rightBottomBorder)

	str := leftBottomBorder
	str += strings.Repeat(horizontalBorder, full)
	str += rightBottomBorder

	return str
}

func strLen(str ...string) int {
	if len(str) > 0 {
		return utf8.RuneCountInString(str[0])
	} else {
		return 0
	}
}
