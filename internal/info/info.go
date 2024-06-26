package info

import (
	"fmt"
	"strings"
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
	topBorder        = "┌──────────────────────────────────────────────────────┐"
	bottomBorder     = "└──────────────────────────────────────────────────────┘"
	sideBorder       = "|"
	horizontalBorder = "─"
	website          = "https://github.com/dobyte/due"
	version          = "v2.1.0"
)

func PrintFrameworkInfo() {
	fmt.Println(strings.TrimSuffix(strings.TrimPrefix(logo, "\n"), "\n"))
	fmt.Println(topBorder)
	fmt.Println(buildRowInfo("Website", website))
	fmt.Println(buildRowInfo("Version", version))
	fmt.Println(bottomBorder)
}

func buildRowInfo(name string, value string) string {
	str := fmt.Sprintf("%s [%s] %s", sideBorder, name, value)
	str += strings.Repeat(" ", utf8.RuneCountInString(topBorder)-utf8.RuneCountInString(str)-1)
	str += sideBorder
	return str
}

type Pair struct {
	Name  string
	Value string
}

// PrintComponentInfo 打印组件信息
func PrintComponentInfo(infos ...string) {
	fmt.Println(topBorder)
	for _, info := range infos {
		fmt.Println(buildPairInfo(info))
	}
	fmt.Println(bottomBorder)
}

func buildPairInfo(info string) string {
	str := fmt.Sprintf("%s %s", sideBorder, info)
	str += strings.Repeat(" ", utf8.RuneCountInString(topBorder)-utf8.RuneCountInString(str)-1)
	str += sideBorder
	return str
}
