package main

import "github.com/charmbracelet/lipgloss"

var (
	colorPulse  = lipgloss.Color("212") // pink — a pulse arriving
	colorShort  = lipgloss.Color("81")  // cyan — short gap / bit 0
	colorLong   = lipgloss.Color("219") // magenta — long gap / bit 1
	colorMuted  = lipgloss.Color("242") // gray — secondary text
	colorGood   = lipgloss.Color("120") // green — success
	colorBad    = lipgloss.Color("203") // red — error
	colorAccent = lipgloss.Color("213") // bright pink — accents/headers

	titleStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	taglineStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	labelStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(colorGood).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorBad).
			Bold(true)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted).
			Padding(0, 1)

	bitZeroStyle = lipgloss.NewStyle().Foreground(colorShort)
	bitOneStyle  = lipgloss.NewStyle().Foreground(colorLong)
	dimStyle     = lipgloss.NewStyle().Foreground(colorMuted)
)

const bannerArt = `
 ██████╗ ██╗   ██╗██╗     ███████╗███████╗████████╗██████╗
 ██╔══██╗██║   ██║██║     ██╔════╝██╔════╝╚══██╔══╝██╔══██╗
 ██████╔╝██║   ██║██║     ███████╗█████╗     ██║   ██████╔╝
 ██╔═══╝ ██║   ██║██║     ╚════██║██╔══╝     ██║   ██╔═══╝
 ██║     ╚██████╔╝███████╗███████║███████╗   ██║   ██║
 ╚═╝      ╚═════╝ ╚══════╝╚══════╝╚══════╝   ╚═╝   ╚═╝
`

func banner() string {
	art := titleStyle.Render(bannerArt)
	tagline := taglineStyle.Render("  Packets carry no data. The silence between them does.")
	return art + "\n" + tagline + "\n"
}
