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

	// maxContentWidth is the maximum display width (in columns) of the content area.
	// It is the total box width minus one column for each of the left and right borders, and
	// minus one leading space reserved before the content.
	maxContentWidth = boxWidth - 3
)

// widthCondition computes terminal display widths.
//
// East Asian width is explicitly disabled so that Ambiguous characters such as box-drawing
// characters count as one column, while Wide/Fullwidth characters such as CJK ones still count as
// two columns.
var widthCondition = &runewidth.Condition{
	EastAsianWidth:     false,
	StrictEmojiNeutral: true,
}

// Print prints the grouped information to standard output.
func Print(name string, rows ...string) {
	Fprint(os.Stdout, name, rows...)
}

// Fprint writes the grouped information to w.
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

// HorizontalLine returns a horizontal separator line used to fill a row.
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
