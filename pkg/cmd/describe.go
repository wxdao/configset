package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/wxdao/configset/pkg/configset"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func NewDescribeCmd(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "describe",
		Short:        "Describe a config set from Kubernetes.",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			setName := args[0]

			restConfig, err := configFlags.ToRESTConfig()
			if err != nil {
				return fmt.Errorf("failed to get rest config: %v", err)
			}

			kubeClient, err := crclient.New(restConfig, crclient.Options{})
			if err != nil {
				return fmt.Errorf("failed to create kube client: %w", err)
			}

			namespace, _, err := configFlags.ToRawKubeConfigLoader().Namespace()
			if err != nil {
				return fmt.Errorf("failed to get namespace: %v", err)
			}

			store, err := configset.NewSecretSetInfoStore(kubeClient, namespace)
			if err != nil {
				return fmt.Errorf("failed to create store: %v", err)
			}

			info, err := store.GetSetInfo(c.Context(), setName)
			if err != nil {
				return fmt.Errorf("failed to get set info: %v", err)
			}

			if info == nil {
				return fmt.Errorf("config set \"%s\" not found", setName)
			}

			tw := tabwriter.NewWriter(c.OutOrStdout(), 0, 0, 2, ' ', 0)
			defer tw.Flush()

			fmt.Fprintf(tw, "Name:\t%s\n", info.Name)
			fmt.Fprintf(tw, "Updated At:\t%s\n", info.UpdatedAt)
			fmt.Fprintf(tw, "No. resources:\t%d\n", len(info.Resources))
			fmt.Fprintf(tw, "Resources:\n")
			for _, r := range info.Resources {
				fmt.Fprintf(tw, "\t%s/%s\n", r.Namespace, r.Name)
				fmt.Fprintf(tw, "\t\tAPIVersion:\t%s\n", r.APIVersion)
				fmt.Fprintf(tw, "\t\tKind:\t%s\n", r.Kind)
				fmt.Fprintf(tw, "\t\tUID:\t%s\n", r.UID)
			}

			return nil
		},
	}

	return cmd
}
