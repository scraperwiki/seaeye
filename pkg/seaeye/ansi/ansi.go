package ansi

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var colors = []string{
	"#000000", "#Dd0000", "#00CF12", "#C2CB00", "#3100CA", "#E100C6", "#00CBCB", "#C7C7C7",
	"#686868", "#FF5959", "#00FF6B", "#FAFF5C", "#775AFF", "#FF47FE", "#0FFFFF", "#FFFFFF",
}

func ToHTML(text []byte) []byte {
	re := regexp.MustCompile("\u001B\\[([0-9A-Za-z;]+)m([^\u001B]+)")
	matches := re.FindAllSubmatch(text, -1)
	if matches == nil {
		return text
	}

	var buf bytes.Buffer

	for _, match := range matches {
		bg, fg := -1, -1
		var bold, underline, negative bool

		codes := bytes.Split(match[1], []byte(";"))
		for _, c := range codes {
			code, _ := strconv.Atoi(string(c))
			if code == 0 {
				bg, fg = -1, -1
				bold, underline, negative = false, false, false
			} else if code == 1 {
				bold = true
			} else if code == 4 {
				underline = true
			} else if code == 7 {
				negative = true
			} else if code == 21 {
				bold = false
			} else if code == 24 {
				underline = false
			} else if code == 27 {
				negative = false
			} else if code >= 30 && code <= 37 {
				fg = code - 30
			} else if code == 39 {
				fg = -1
			} else if code >= 40 && code <= 47 {
				bg = code - 40
			} else if code == 49 {
				bg = -1
			} else if code >= 90 && code <= 97 {
				fg = code - 90 + 8
			} else if code >= 100 && code <= 107 {
				bg = code - 100 + 8
			}
		}

		style := ""
		if negative {
			fg = bg
			bg = fg
		}
		if bold {
			fg = fg | 8
			style += "font-weight: bold;"
		}
		if underline {
			style += "text-decoration:underline"
		}
		if fg >= 0 {
			style += fmt.Sprintf("color: %s;", colors[fg])
		}
		if bg >= 0 {
			style += fmt.Sprintf("background-color: %s;", colors[bg])
		}

		html := string(match[2])
		html = strings.Replace(html, "&", "&amp;", -1)
		html = strings.Replace(html, "<", "&lt;", -1)
		html = strings.Replace(html, ">", "&gt;", -1)

		if style == "" {
			buf.WriteString(html)
		} else {
			buf.WriteString(fmt.Sprintf(`<span style="%s">%s</span>`, style, html))
		}
	}

	return buf.Bytes()
}
