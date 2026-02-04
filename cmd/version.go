package cmd

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

var (
	// Default values - overridden by ldflags during goreleaser builds
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

func init() {
	rootCmd.AddCommand(versionCmd)

	// Try to get version from build info (works with go install @version)
	if info, ok := debug.ReadBuildInfo(); ok {
		// Module version (e.g., "v0.5.1" from go install @v0.5.1 or @latest)
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			ver := strings.TrimPrefix(info.Main.Version, "v")
			if Version == "dev" {
				Version = ver
			}
		}

		// VCS info from build settings
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if GitCommit == "unknown" && len(setting.Value) >= 7 {
					GitCommit = setting.Value[:7]
				}
			case "vcs.time":
				if BuildDate == "unknown" {
					BuildDate = setting.Value
				}
			}
		}
	}
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("whatap-go-inst version v%s\n", Version)
		fmt.Printf("  Git commit: %s\n", GitCommit)
		fmt.Printf("  Build date: %s\n", BuildDate)
	},
}
