package cleanup

import (
	"github.com/spf13/cobra"
)

// CleanUpCmd represents the clean up command
var CleanUpCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up resources",
	Long:  "Cleans up all resources that are in finished state or not needed",
}

func init() {
	CleanUpCmd.AddCommand(podsCmd)
}