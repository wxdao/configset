package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/wxdao/configset/pkg/configset"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func NewListCmd(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List all config sets from Kubernetes.",
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			restConfig, err := configFlags.ToRESTConfig()
			if err != nil {
				return fmt.Errorf("failed to get rest config: %v", err)
			}

			namespace, _, err := configFlags.ToRawKubeConfigLoader().Namespace()
			if err != nil {
				return fmt.Errorf("failed to get namespace: %v", err)
			}

			store, err := configset.NewSecretSetInfoStore(restConfig, namespace)
			if err != nil {
				return fmt.Errorf("failed to create store: %v", err)
			}

			infos, err := store.ListSetInfos(c.Context())
			if err != nil {
				return fmt.Errorf("failed to list set infos: %v", err)
			}

			tw := tabwriter.NewWriter(c.OutOrStdout(), 0, 0, 5, ' ', 0)
			defer tw.Flush()

			tw.Write([]byte("NAME\tNO. RESOURCES\tUPDATED AT\n"))
			for _, info := range infos {
				fmt.Fprintf(tw, "%s\t%d\t%s\n", info.Name, len(info.Resources), info.UpdatedAt)
			}

			return nil
		},
	}

	return cmd
}
