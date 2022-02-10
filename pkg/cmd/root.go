package cmd

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func NewRootCmd() *cobra.Command {
	configFlags := genericclioptions.NewConfigFlags(true)

	cmd := &cobra.Command{
		Use:          "configset",
		Short:        "Management of config sets for Kubernetes.",
		SilenceUsage: true,
	}

	configFlags.AddFlags(cmd.PersistentFlags())

	cmd.AddCommand(NewApplyCmd(configFlags))
	cmd.AddCommand(NewDeleteCmd(configFlags))
	cmd.AddCommand(NewListCmd(configFlags))
	cmd.AddCommand(NewDescribeCmd(configFlags))

	return cmd
}
