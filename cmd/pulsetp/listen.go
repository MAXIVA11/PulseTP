package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/MAXIVA11/PulseTP/pulsetp"
)

func newListenCmd() *cobra.Command {
	var (
		port       int
		threshold  time.Duration
		endSilence time.Duration
		timeout    time.Duration
		plain      bool
		output     string
		key        string
	)

	cmd := &cobra.Command{
		Use:   "listen",
		Short: "Listen for an incoming pulse train and decode it",
		Example: `  pulsetp listen --port 9000
  pulsetp listen --port 9000 --output ./received.png
  pulsetp listen --port 9000 --key "correct horse battery staple"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := pulsetp.DefaultConfig()
			cfg.Threshold = threshold
			cfg.EndSilence = endSilence

			l, err := pulsetp.Listen(cfg, port)
			if err != nil {
				return fmt.Errorf("could not start listening: %w", err)
			}
			defer l.Close()

			ctx := cmd.Context()
			if timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}
			ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
			defer stop()

			if key == "" {
				key = os.Getenv("PULSETP_KEY")
			}

			if plain || !isatty.IsTerminal(os.Stdout.Fd()) {
				return runListenPlain(ctx, l, port, output, key)
			}
			return runListenTUI(ctx, l, port, output, key)
		},
	}

	cmd.Flags().IntVar(&port, "port", 9000, "UDP port to listen on")
	cmd.Flags().DurationVar(&threshold, "threshold", 100*time.Millisecond, "fallback 0/1 threshold, used only until the preamble calibrates it")
	cmd.Flags().DurationVar(&endSilence, "end-silence", 2*time.Second, "silence duration that marks end-of-message")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "give up if no pulses arrive within this long (0 = wait forever)")
	cmd.Flags().BoolVar(&plain, "plain", false, "disable the live TUI, print plain text as pulses decode")
	cmd.Flags().StringVarP(&output, "output", "o", "", "save the decoded bytes to this file instead of printing them as text (for receiving files)")
	cmd.Flags().StringVarP(&key, "key", "k", "", "decrypt received data with this passphrase (must match the sender's --key); can also be set via PULSETP_KEY")

	return cmd
}

func runListenPlain(ctx context.Context, l *pulsetp.Listener, port int, output, key string) error {
	fmt.Println(labelStyle.Render("listening  ") + valueStyle.Render(fmt.Sprintf("udp/%d", port)))
	fmt.Println(dimStyle.Render("waiting for pulses... (ctrl+c to stop)"))
	fmt.Println()

	start := time.Now()
	for ev := range l.Run(ctx) {
		switch ev.Kind {
		case pulsetp.EventPulse:
			printPulsePlain(ev.Pulse)
		case pulsetp.EventCalibrated:
			fmt.Println()
			c := ev.Calibration
			fmt.Println(dimStyle.Render(fmt.Sprintf(
				"  calibrated: short≈%s long≈%s threshold=%s tolerance=±%s",
				c.AvgShort.Round(time.Millisecond), c.AvgLong.Round(time.Millisecond),
				c.Threshold.Round(time.Millisecond), c.Tolerance.Round(time.Millisecond))))
			fmt.Print(labelStyle.Render("data       "))
		case pulsetp.EventByte:
			// pulse printing already shows bits; nothing extra needed here.
		case pulsetp.EventError:
			fmt.Fprintln(os.Stderr, "\n"+errorStyle.Render("! ")+ev.Err.Error())
		case pulsetp.EventMessage:
			fmt.Println()
			fmt.Println()
			if len(ev.Message) == 0 {
				fmt.Println(errorStyle.Render("✗ no message decoded (silence before any data arrived)"))
				return nil
			}
			decoded := ev.Message
			if key != "" {
				plain, err := pulsetp.Decrypt(key, decoded)
				if err != nil {
					fmt.Println(errorStyle.Render("✗ " + err.Error()))
					return nil
				}
				decoded = plain
			}
			ev.Message = decoded
			elapsed := labelStyle.Render(fmt.Sprintf("  in %s", time.Since(start).Round(time.Millisecond)))
			if output != "" {
				if err := os.WriteFile(output, ev.Message, 0o644); err != nil {
					return fmt.Errorf("could not save to %q: %w", output, err)
				}
				fmt.Println(successStyle.Render("✓ file received") + elapsed)
				fmt.Println(panelStyle.Render(valueStyle.Render(fmt.Sprintf("saved %s to %s", humanBytes(len(ev.Message)), output))))
				return nil
			}
			fmt.Println(successStyle.Render("✓ message received") + elapsed)
			fmt.Println(panelStyle.Render(valueStyle.Render(string(ev.Message))))
			return nil
		}
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("stopped before a message completed: %w", err)
	}
	return nil
}

func printPulsePlain(p pulsetp.PulseInfo) {
	switch {
	case p.Index == 0:
		fmt.Print(dimStyle.Render("● "))
	case p.Phase == pulsetp.PhasePreamble:
		fmt.Print(dimStyle.Render("┆ "))
	case !p.HasBit:
		fmt.Print(dimStyle.Render("· "))
	case p.Bit == 0:
		fmt.Print(bitZeroStyle.Render("▪ "))
	default:
		fmt.Print(bitOneStyle.Render("▮ "))
	}
}
