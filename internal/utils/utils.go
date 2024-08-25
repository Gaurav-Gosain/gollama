package utils

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	charmansi "github.com/charmbracelet/x/exp/term/ansi"
	"github.com/mattn/go-runewidth"
	ansi "github.com/muesli/reflow/ansi"

	"github.com/muesli/reflow/truncate"
	"github.com/muesli/termenv"
)

// Pretty prints an error to stderr and exits the program if exitOnErr is true
func PrintError(err error, exitOnErr bool) {
	ErrPadding := lipgloss.NewStyle().Padding(1, 2)
	ErrorHeader := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F1F1F1")).
		Background(lipgloss.Color("#FF5F87")).
		Bold(true).
		Padding(0, 1).
		SetString("ERROR")

	if err != nil {
		fmt.Fprintln(
			os.Stderr,
			ErrPadding.Render(
				fmt.Sprintf(
					"\n%s %s",
					ErrorHeader.String(),
					err.Error(),
				),
			),
		)
		if exitOnErr {
			os.Exit(1)
		}
	}
}

/*
* Thanks to https://github.com/charmbracelet/lipgloss/pull/102 for the implementation!
* */

// whitespace is a whitespace renderer.
type whitespace struct {
	chars string
	style termenv.Style
}

type WhitespaceOption func(*whitespace)

// Render whitespaces.
func (w whitespace) render(width int) string {
	if w.chars == "" {
		w.chars = " "
	}

	r := []rune(w.chars)
	j := 0
	b := strings.Builder{}

	// Cycle through runes and print them into the whitespace.
	for i := 0; i < width; {
		b.WriteRune(r[j])
		j++
		if j >= len(r) {
			j = 0
		}
		i += charmansi.StringWidth(string(r[j]))
	}

	// Fill any extra gaps white spaces. This might be necessary if any runes
	// are more than one cell wide, which could leave a one-rune gap.
	short := width - charmansi.StringWidth(b.String())
	if short > 0 {
		b.WriteString(strings.Repeat(" ", short))
	}

	return w.style.Styled(b.String())
}

// PlaceOverlay places fg on top of bg.
func PlaceOverlay(x, y int, fg, bg string, opts ...WhitespaceOption) string {
	fgLines, fgWidth := getLines(fg)
	bgLines, bgWidth := getLines(bg)
	bgHeight := len(bgLines)
	fgHeight := len(fgLines)

	if fgWidth >= bgWidth && fgHeight >= bgHeight {
		// FIXME: return fg or bg?
		return fg
	}
	// TODO: allow placement outside of the bg box?
	x = clamp(x, 0, bgWidth-fgWidth)
	y = clamp(y, 0, bgHeight-fgHeight)

	ws := &whitespace{}
	for _, opt := range opts {
		opt(ws)
	}

	var b strings.Builder
	for i, bgLine := range bgLines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if i < y || i >= y+fgHeight {
			b.WriteString(bgLine)
			continue
		}

		pos := 0
		if x > 0 {
			left := truncate.String(bgLine, uint(x))
			pos = ansi.PrintableRuneWidth(left)
			b.WriteString(left)
			if pos < x {
				b.WriteString(ws.render(x - pos))
				pos = x
			}
		}

		fgLine := fgLines[i-y]
		b.WriteString(fgLine)
		pos += ansi.PrintableRuneWidth(fgLine)

		right := cutLeft(bgLine, pos)
		bgWidth := ansi.PrintableRuneWidth(bgLine)
		rightWidth := ansi.PrintableRuneWidth(right)
		if rightWidth <= bgWidth-pos {
			b.WriteString(ws.render(bgWidth - rightWidth - pos))
		}

		b.WriteString(right)
	}

	return b.String()
}

// cutLeft cuts printable characters from the left.
// This function is heavily based on muesli's ansi and truncate packages.
func cutLeft(s string, cutWidth int) string {
	var (
		pos    int
		isAnsi bool
		ab     bytes.Buffer
		b      bytes.Buffer
	)
	for _, c := range s {
		var w int
		if c == ansi.Marker || isAnsi {
			isAnsi = true
			ab.WriteRune(c)
			if ansi.IsTerminator(c) {
				isAnsi = false
				if bytes.HasSuffix(ab.Bytes(), []byte("[0m")) {
					ab.Reset()
				}
			}
		} else {
			w = runewidth.RuneWidth(c)
		}

		if pos >= cutWidth {
			if b.Len() == 0 {
				if ab.Len() > 0 {
					b.Write(ab.Bytes())
				}
				if pos-cutWidth > 1 {
					b.WriteByte(' ')
					continue
				}
			}
			b.WriteRune(c)
		}
		pos += w
	}
	return b.String()
}

func clamp(v, lower, upper int) int {
	return min(max(v, lower), upper)
}

// Split a string into lines, additionally returning the size of the widest
// line.
func getLines(s string) (lines []string, widest int) {
	lines = strings.Split(s, "\n")

	for _, l := range lines {
		w := charmansi.StringWidth(l)
		if widest < w {
			widest = w
		}
	}

	return lines, widest
}

// ExpandPath expands the tilde (~) in a path to the user's home directory
func ExpandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		usr, err := user.Current()
		if err != nil {
			return "", err
		}
		path = filepath.Join(usr.HomeDir, path[2:])
	}
	return path, nil
}

func GetImageWidth(path string, physicalWidth int, physicalHeight int) (int, image.Image, error) {
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return 0, nil, err
	}

	file, err := os.Open(expandedPath)
	if err != nil {
		return 0, nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return 0, nil, err
	}

	ratio := float64(img.Bounds().Dy()) / float64(img.Bounds().Dx())

	newHeight := (physicalHeight * 2)
	if newHeight > 4 {
		newHeight -= 4
	}
	width := int(float64(newHeight) / ratio)
	return width, img, nil
}
