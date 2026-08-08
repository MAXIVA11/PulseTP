package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/MAXIVA11/PulseTP/pulsetp"
)

func newBenchCmd() *cobra.Command {
	var (
		port     int
		shortGap time.Duration
		longGap  time.Duration
		size     int
	)

	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Benchmark PulseTP throughput with a local loopback send/receive",
		Long: "Benchmark spins up a listener on 127.0.0.1, sends a synthetic message of --size\n" +
			"bytes to it, and reports how many bits per second the rhythm actually achieved.",
		Example: `  pulsetp bench --size 32`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := pulsetp.DefaultConfig()
			cfg.ShortGap = shortGap
			cfg.LongGap = longGap
			cfg.Threshold = (shortGap + longGap) / 2
			cfg.EndSilence = 1 * time.Second

			l, err := pulsetp.Listen(cfg, port)
			if err != nil {
				return fmt.Errorf("could not start listener: %w", err)
			}
			defer l.Close()

			addr := fmt.Sprintf("127.0.0.1:%d", port)
			payload := make([]byte, size)
			for i := range payload {
				payload[i] = byte('a' + i%26)
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			fmt.Println(labelStyle.Render("payload    ") + valueStyle.Render(fmt.Sprintf("%d bytes", size)))
			fmt.Println(labelStyle.Render("gaps       ") +
				bitZeroStyle.Render(fmt.Sprintf("0=%s", shortGap)) + "  " +
				bitOneStyle.Render(fmt.Sprintf("1=%s", longGap)))
			fmt.Println(dimStyle.Render("running local loopback benchmark..."))

			events := l.Run(ctx)
			sendErrCh := make(chan error, 1)
			start := time.Now()
			go func() {
				sendErrCh <- pulsetp.Send(ctx, addr, payload, cfg, nil)
			}()

			var decoded []byte
			var pulseCount int
			for ev := range events {
				if ev.Kind == pulsetp.EventPulse {
					pulseCount++
				}
				if ev.Kind == pulsetp.EventMessage {
					decoded = ev.Message
				}
				if ev.Kind == pulsetp.EventError {
					return fmt.Errorf("benchmark receive error: %w", ev.Err)
				}
			}
			elapsed := time.Since(start)

			if err := <-sendErrCh; err != nil {
				return fmt.Errorf("benchmark send error: %w", err)
			}
			if string(decoded) != string(payload) {
				return fmt.Errorf("decoded message did not match what was sent (got %d bytes, want %d)", len(decoded), len(payload))
			}

			bits := len(decoded) * 8
			bitsPerSec := float64(bits) / elapsed.Seconds()

			fmt.Println()
			fmt.Println(successStyle.Render("✓ ok") + labelStyle.Render(fmt.Sprintf("  %d pulses, %s total", pulseCount, elapsed.Round(time.Millisecond))))
			fmt.Println(labelStyle.Render("throughput ") + valueStyle.Render(fmt.Sprintf("%.1f bits/sec  (%.2f bytes/sec)", bitsPerSec, bitsPerSec/8)))
			return nil
		},
	}

	cmd.Flags().IntVar(&port, "port", 9000, "loopback UDP port to use")
	cmd.Flags().DurationVar(&shortGap, "short", 40*time.Millisecond, "nominal gap encoding bit 0")
	cmd.Flags().DurationVar(&longGap, "long", 160*time.Millisecond, "nominal gap encoding bit 1")
	cmd.Flags().IntVar(&size, "size", 16, "size in bytes of the synthetic benchmark payload")

	return cmd
}
