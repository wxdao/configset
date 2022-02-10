package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/wxdao/configset/pkg/configset"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func NewDeleteCmd(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	dryRunFlag := false

	cmd := &cobra.Command{
		Use:          "delete",
		Short:        "Delete a config set from Kubernetes.",
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("must specify a config set name")
			}
			setName := args[0]

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

			cli, err := configset.NewClient(configset.ClientOptions{
				RESTConfig: restConfig,
				Store:      store,
			})
			if err != nil {
				return fmt.Errorf("failed to create configset client: %v", err)
			}

			res, err := cli.Delete(c.Context(), setName, configset.DeleteOptions{
				DryRun: dryRunFlag,
				LogObjectFunc: func(obj configset.Object, action configset.LogObjectAction, err error) {
					gvk := obj.GetObjectKind().GroupVersionKind()
					apiVersion := gvk.Group + "/" + gvk.Version
					if gvk.Group == "" {
						apiVersion = gvk.Version
					}
					if err == nil {
						err = fmt.Errorf("ok")
					}
					log.Printf("%s: ns=%s name=%s apiVersion=%s kind=%s: %v", action, obj.GetNamespace(), obj.GetName(), apiVersion, gvk.Kind, err)
				},
			})
			_ = res
			return err
		},
	}

	cmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "If true, submit server-side request without persisting the resource.")

	return cmd
}
