package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/MAXIVA11/PulseTP/pulsetp"
)

func newSendCmd() *cobra.Command {
	var (
		to       string
		message  string
		shortGap time.Duration
		longGap  time.Duration
		plain    bool
	)

	cmd := &cobra.Command{
		Use:     "send",
		Short:   "Send a message as a rhythm of UDP pulses",
		Example: `  pulsetp send --to localhost:9000 --message "hello"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if message == "" {
				return fmt.Errorf(`--message is required, e.g. --message "hello"`)
			}
			if shortGap <= 0 || longGap <= shortGap {
				return fmt.Errorf("--long must be greater than --short (got short=%s long=%s)", shortGap, longGap)
			}

			cfg := pulsetp.DefaultConfig()
			cfg.ShortGap = shortGap
			cfg.LongGap = longGap
			cfg.Threshold = (shortGap + longGap) / 2

			totalBits := len(cfg.PreambleBits) + len(message)*8
			fmt.Println(labelStyle.Render("target     ") + valueStyle.Render(to))
			fmt.Println(labelStyle.Render("message    ") + valueStyle.Render(fmt.Sprintf("%q", message)))
			fmt.Println(labelStyle.Render("gaps       ") +
				bitZeroStyle.Render(fmt.Sprintf("0=%s", shortGap)) + "  " +
				bitOneStyle.Render(fmt.Sprintf("1=%s", longGap)))
			fmt.Println(labelStyle.Render("pulses     ") + valueStyle.Render(fmt.Sprintf("%d", totalBits+1)))
			fmt.Println()

			if !plain {
				fmt.Print(labelStyle.Render("rhythm     "))
			}

			start := time.Now()
			err := pulsetp.Send(cmd.Context(), to, []byte(message), cfg, func(p pulsetp.SentPulse) {
				if plain {
					return
				}
				switch {
				case p.Index == 0:
					fmt.Print(dimStyle.Render("● "))
				case p.Phase == pulsetp.PhasePreamble:
					fmt.Print(dimStyle.Render("┆ "))
				case p.Bit == 0:
					fmt.Print(bitZeroStyle.Render("▪ "))
				default:
					fmt.Print(bitOneStyle.Render("▮ "))
				}
			})
			if !plain {
				fmt.Println()
			}
			if err != nil {
				return fmt.Errorf("send failed: %w", err)
			}

			fmt.Println()
			fmt.Println(successStyle.Render("✓ sent") +
				labelStyle.Render(fmt.Sprintf("  in %s", time.Since(start).Round(time.Millisecond))))
			return nil
		},
	}

	cmd.Flags().StringVar(&to, "to", "localhost:9000", "destination address (host:port)")
	cmd.Flags().StringVarP(&message, "message", "m", "", `message to send, e.g. --message "hello"`)
	cmd.Flags().DurationVar(&shortGap, "short", 40*time.Millisecond, "nominal gap encoding bit 0")
	cmd.Flags().DurationVar(&longGap, "long", 160*time.Millisecond, "nominal gap encoding bit 1")
	cmd.Flags().BoolVar(&plain, "plain", false, "disable live rhythm output, print a plain summary")

	return cmd
}
