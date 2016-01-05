package dxf

import (
	"bufio"
	"fmt"
	"github.com/yofu/dxf/color"
	"github.com/yofu/dxf/drawing"
	"github.com/yofu/dxf/table"
	"os"
	"strings"
)

var (
	DefaultColor    = color.White
	DefaultLineType = table.LT_CONTINUOUS
)

func NewDrawing() *drawing.Drawing {
	return drawing.New()
}

func Open(filename string) (*drawing.Drawing, error) {
	var err error
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(f)
	d := NewDrawing()
	var code, value string
	var reader drawing.Section
	data := make([][2]string, 0)
	setreader := false
	line := 0
	startline := 0
	for scanner.Scan() {
		line++
		if line%2 == 1 {
			code = strings.TrimSpace(scanner.Text())
			if err != nil {
				return d, err
			}
		} else {
			value = scanner.Text()
			if setreader {
				if code != "2" {
					return d, fmt.Errorf("line %d: invalid group code: %s", line, code)
				}
				ind := drawing.SectionTypeValue(strings.ToUpper(value))
				if ind < 0 {
					return d, fmt.Errorf("line %d: unknown section name: %s", line, value)
				}
				reader = d.Sections[ind]
				startline = line + 1
				setreader = false
			} else {
				if code == "0" {
					switch strings.ToUpper(value) {
					case "EOF":
						return d, nil
					case "SECTION":
						setreader = true
					case "ENDSEC":
						err := reader.Read(startline, data)
						if err != nil {
							return d, err
						}
						data = make([][2]string, 0)
						startline = line + 1
					default:
						data = append(data, [2]string{code, scanner.Text()})
					}
				} else {
					data = append(data, [2]string{code, scanner.Text()})
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return d, err
	}
	if len(data) > 0 {
		err := reader.Read(startline, data)
		if err != nil {
			return d, err
		}
	}
	return d, nil
}

func ColorIndex(cl []int) color.ColorNumber {
	minind := 0
	minval := 1000000
	for i, c := range color.ColorRGB {
		tmpval := 0
		for j := 0; j < 3; j++ {
			tmpval += (cl[j] - int(c[j])) * (cl[j] - int(c[j]))
		}
		if tmpval < minval {
			minind = i
			minval = tmpval
			if minval == 0 {
				break
			}
		}
	}
	return color.ColorNumber(minind)
}

func IndexColor(index uint8) []uint8 {
	return color.ColorRGB[index]
}
