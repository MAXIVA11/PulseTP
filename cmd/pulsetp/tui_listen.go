package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/MAXIVA11/PulseTP/pulsetp"
)

type listenEventMsg pulsetp.Event
type listenClosedMsg struct{}

func waitForListenEvent(ch <-chan pulsetp.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return listenClosedMsg{}
		}
		return listenEventMsg(ev)
	}
}

type listenModel struct {
	port   int
	events <-chan pulsetp.Event
	cancel context.CancelFunc

	spinner spinner.Model
	phase   pulsetp.Phase

	glyphs []string // one rendered glyph per received pulse

	calibrated  bool
	calibration pulsetp.Calibration

	message    []byte
	outputPath string
	saveErr    error
	lastErr    error

	start time.Time
	done  bool
	quit  bool
}

func newListenModel(ctx context.Context, l *pulsetp.Listener, port int, outputPath string) (listenModel, context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	sp := spinner.New()
	sp.Spinner = spinner.Points
	sp.Style = dimStyle
	return listenModel{
		port:       port,
		events:     l.Run(runCtx),
		cancel:     cancel,
		spinner:    sp,
		phase:      pulsetp.PhaseWaiting,
		outputPath: outputPath,
		start:      time.Now(),
	}, runCtx
}

func (m listenModel) Init() tea.Cmd {
	return tea.Batch(waitForListenEvent(m.events), m.spinner.Tick)
}

func (m listenModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.cancel()
			m.quit = true
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case listenEventMsg:
		ev := pulsetp.Event(msg)
		switch ev.Kind {
		case pulsetp.EventPulse:
			m.phase = ev.Pulse.Phase
			m.glyphs = append(m.glyphs, pulseGlyph(ev.Pulse))
		case pulsetp.EventCalibrated:
			m.calibrated = true
			m.calibration = ev.Calibration
		case pulsetp.EventByte:
			m.message = ev.Message
		case pulsetp.EventError:
			m.lastErr = ev.Err
		case pulsetp.EventMessage:
			m.message = ev.Message
			m.done = true
			if m.outputPath != "" && len(ev.Message) > 0 {
				m.saveErr = os.WriteFile(m.outputPath, ev.Message, 0o644)
			}
			m.cancel()
			return m, tea.Quit
		}
		return m, waitForListenEvent(m.events)

	case listenClosedMsg:
		m.done = true
		return m, tea.Quit
	}

	return m, nil
}

func pulseGlyph(p pulsetp.PulseInfo) string {
	switch {
	case p.Index == 0:
		return dimStyle.Render("●")
	case p.Phase == pulsetp.PhasePreamble:
		return dimStyle.Render("┆")
	case !p.HasBit:
		return dimStyle.Render("·")
	case p.Bit == 0:
		style := bitZeroStyle
		if !p.Confident {
			style = style.Faint(true)
		}
		return style.Render("▪")
	default:
		style := bitOneStyle
		if !p.Confident {
			style = style.Faint(true)
		}
		return style.Render("▮")
	}
}

func (m listenModel) View() string {
	var b strings.Builder

	fmt.Fprintln(&b, titleStyle.Render("PulseTP")+dimStyle.Render(fmt.Sprintf("  listening on udp/%d", m.port)))
	b.WriteString("\n")

	status := m.statusLine()
	fmt.Fprintln(&b, status)
	b.WriteString("\n")

	if m.calibrated {
		c := m.calibration
		fmt.Fprintln(&b, dimStyle.Render(fmt.Sprintf(
			"calibrated  short≈%s  long≈%s  threshold=%s  tolerance=±%s",
			c.AvgShort.Round(time.Millisecond), c.AvgLong.Round(time.Millisecond),
			c.Threshold.Round(time.Millisecond), c.Tolerance.Round(time.Millisecond))))
		b.WriteString("\n")
	}

	fmt.Fprintln(&b, labelStyle.Render("rhythm"))
	fmt.Fprintln(&b, strings.Join(m.glyphs, " "))
	b.WriteString("\n")

	if m.outputPath != "" {
		fmt.Fprintln(&b, labelStyle.Render("file"))
		var content string
		switch {
		case m.saveErr != nil:
			content = errorStyle.Render("save failed: " + m.saveErr.Error())
		case m.done:
			content = valueStyle.Render(fmt.Sprintf("saved %s to %s", humanBytes(len(m.message)), m.outputPath))
		case len(m.message) == 0:
			content = dimStyle.Render("(waiting for data...)")
		default:
			content = valueStyle.Render(fmt.Sprintf("%s received", humanBytes(len(m.message)))) + dimStyle.Render("▌")
		}
		fmt.Fprintln(&b, panelStyle.Render(content))
	} else {
		fmt.Fprintln(&b, labelStyle.Render("decoded"))
		content := string(m.message)
		if content == "" {
			content = dimStyle.Render("(waiting for data...)")
		} else if !m.done {
			content += dimStyle.Render("▌")
		}
		fmt.Fprintln(&b, panelStyle.Render(content))
	}

	if m.lastErr != nil {
		b.WriteString("\n")
		fmt.Fprintln(&b, errorStyle.Render("! ")+m.lastErr.Error())
	}

	b.WriteString("\n")
	if m.done {
		b.WriteString(successStyle.Render("✓ done") + dimStyle.Render(fmt.Sprintf("  in %s", time.Since(m.start).Round(time.Millisecond))))
	} else {
		b.WriteString(dimStyle.Render("q / ctrl+c to stop"))
	}
	b.WriteString("\n")

	return b.String()
}

func (m listenModel) statusLine() string {
	switch m.phase {
	case pulsetp.PhaseWaiting:
		return m.spinner.View() + " " + dimStyle.Render("waiting for the first pulse...")
	case pulsetp.PhasePreamble:
		return m.spinner.View() + " " + dimStyle.Render("calibrating against preamble...")
	default:
		return successStyle.Render("● ") + dimStyle.Render("decoding data")
	}
}

func runListenTUI(ctx context.Context, l *pulsetp.Listener, port int, outputPath string) error {
	model, _ := newListenModel(ctx, l, port, outputPath)
	p := tea.NewProgram(model)
	final, err := p.Run()
	if err != nil {
		return fmt.Errorf("tui error: %w", err)
	}

	m := final.(listenModel)
	if m.quit && !m.done {
		return fmt.Errorf("stopped before a message completed")
	}
	if m.done && len(m.message) == 0 {
		return fmt.Errorf("no message decoded (silence before any data arrived)")
	}
	if m.saveErr != nil {
		return fmt.Errorf("could not save to %q: %w", m.outputPath, m.saveErr)
	}
	return nil
}
