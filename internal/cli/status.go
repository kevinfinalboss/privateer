package cli

import (
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Mostra status das operações",
	Long:  "Exibe informações sobre o status atual das operações do Privateer",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Info("operation_completed").
			Str("operation", "status").
			Send()
		return nil
	},
}
