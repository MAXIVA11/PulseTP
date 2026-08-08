// Command pulsetp sends and receives messages encoded entirely in the
// timing between UDP packets — the rhythm-based PulseTP protocol.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, errorStyle.Render("pulsetp crashed: ")+fmt.Sprint(r))
			os.Exit(1)
		}
	}()

	root := newRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, errorStyle.Render("error: ")+humanize(err))
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "pulsetp",
		Short:         "PulseTP — a rhythm-based protocol where silence carries the message",
		Long:          banner() + "\n" + taglineStyle.Render("  Morse code for the network age: gaps between packets, not their contents, are the payload."),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newSendCmd())
	root.AddCommand(newListenCmd())
	root.AddCommand(newBenchCmd())

	return root
}

// humanize turns internal errors into short, friendly messages instead of
// letting raw Go error chains (or panics) reach the user.
func humanize(err error) string {
	return err.Error()
}
