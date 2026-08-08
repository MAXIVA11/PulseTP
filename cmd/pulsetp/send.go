package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/MAXIVA11/PulseTP/pulsetp"
)

func newSendCmd() *cobra.Command {
	var (
		to       string
		message  string
		file     string
		key      string
		shortGap time.Duration
		longGap  time.Duration
		repeat   int
		plain    bool
	)

	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send a message or a file as a rhythm of UDP pulses",
		Example: `  pulsetp send --to localhost:9000 --message "hello"
  pulsetp send --to localhost:9000 --file ./photo.png
  pulsetp send --to localhost:9000 --message "secret" --key "correct horse battery staple"
  pulsetp send --to localhost:9000 --message "hello" --repeat 3`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if message != "" && file != "" {
				return fmt.Errorf("--message and --file are mutually exclusive, pick one")
			}
			if message == "" && file == "" {
				return fmt.Errorf(`--message or --file is required, e.g. --message "hello"`)
			}
			if shortGap <= 0 || longGap <= shortGap {
				return fmt.Errorf("--long must be greater than --short (got short=%s long=%s)", shortGap, longGap)
			}
			if repeat < 1 || repeat%2 == 0 {
				return fmt.Errorf("--repeat must be an odd number >= 1 so a majority vote never ties (got %d)", repeat)
			}

			var payload []byte
			var label string
			if file != "" {
				data, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("could not read %q: %w", file, err)
				}
				payload = data
				label = fmt.Sprintf("%s (%s)", filepath.Base(file), humanBytes(len(data)))
			} else {
				payload = []byte(message)
				label = fmt.Sprintf("%q", message)
			}

			if key == "" {
				key = os.Getenv("PULSETP_KEY")
			}
			if key != "" {
				encrypted, err := pulsetp.Encrypt(key, payload)
				if err != nil {
					return fmt.Errorf("could not encrypt payload: %w", err)
				}
				payload = encrypted
			}

			cfg := pulsetp.DefaultConfig()
			cfg.ShortGap = shortGap
			cfg.LongGap = longGap
			cfg.Threshold = (shortGap + longGap) / 2
			cfg.Repeat = repeat

			totalBits := len(cfg.PreambleBits) + len(payload)*8*repeat
			avgGap := (shortGap + longGap) / 2
			estimate := time.Duration(totalBits) * avgGap

			targetLabel := "message    "
			if file != "" {
				targetLabel = "file       "
			}

			fmt.Println(labelStyle.Render("target     ") + valueStyle.Render(to))
			fmt.Println(labelStyle.Render(targetLabel) + valueStyle.Render(label))
			fmt.Println(labelStyle.Render("gaps       ") +
				bitZeroStyle.Render(fmt.Sprintf("0=%s", shortGap)) + "  " +
				bitOneStyle.Render(fmt.Sprintf("1=%s", longGap)))
			fmt.Println(labelStyle.Render("pulses     ") + valueStyle.Render(fmt.Sprintf("%d", totalBits+1)))
			fmt.Println(labelStyle.Render("estimated  ") + dimStyle.Render(fmt.Sprintf("~%s", estimate.Round(time.Millisecond*100))))
			if key != "" {
				fmt.Println(labelStyle.Render("encrypted  ") + successStyle.Render("✓ AES-256-GCM"))
			}
			if repeat > 1 {
				fmt.Println(labelStyle.Render("repeat     ") + dimStyle.Render(fmt.Sprintf("%dx per bit (majority vote)", repeat)))
			}
			fmt.Println()

			if !plain {
				fmt.Print(labelStyle.Render("rhythm     "))
			}

			start := time.Now()
			err := pulsetp.Send(cmd.Context(), to, payload, cfg, func(p pulsetp.SentPulse) {
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
	cmd.Flags().StringVarP(&file, "file", "f", "", "path to a file to send instead of a text message")
	cmd.Flags().StringVarP(&key, "key", "k", "", "encrypt the payload with this passphrase (AES-256-GCM); can also be set via PULSETP_KEY. Warning: passing it directly may leave it in your shell history")
	cmd.Flags().DurationVar(&shortGap, "short", 40*time.Millisecond, "nominal gap encoding bit 0")
	cmd.Flags().DurationVar(&longGap, "long", 160*time.Millisecond, "nominal gap encoding bit 1")
	cmd.Flags().IntVar(&repeat, "repeat", 1, "send each data bit this many times (must be odd); listen --repeat must match")
	cmd.Flags().BoolVar(&plain, "plain", false, "disable live rhythm output, print a plain summary")

	return cmd
}

func humanBytes(n int) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
