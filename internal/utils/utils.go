package utils

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
)

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
