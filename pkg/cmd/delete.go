package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wxdao/configset/pkg/configset"
	"github.com/wxdao/configset/pkg/diffutil"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func NewDeleteCmd(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	dryRunFlag := false
	diffFlag := false
	stripManagedFieldsFlag := false

	cmd := &cobra.Command{
		Use:          "delete <name>",
		Short:        "Delete a config set from Kubernetes.",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			setName := args[0]

			if diffFlag {
				dryRunFlag = true
			}

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

			cli, err := configset.NewClient(restConfig, store)
			if err != nil {
				return fmt.Errorf("failed to create configset client: %v", err)
			}

			res, err := cli.Delete(c.Context(), setName, configset.DeleteOptions{
				DryRun: dryRunFlag,
				LogObjectResultFunc: func(objRes configset.ObjectResult) {
					gvk := objRes.Config.GetObjectKind().GroupVersionKind()
					kind := strings.ToLower(gvk.Kind)
					if gvk.Group != "" {
						kind = kind + "." + strings.ToLower(gvk.Group)
					}
					errStr := ""
					if objRes.Error != nil {
						errStr = fmt.Sprintf(" - error: %s", objRes.Error.Error())
					}
					fmt.Fprintf(c.OutOrStdout(), "%s: %s/%s%s\n", objRes.Action, kind, objRes.Config.GetName(), errStr)
				},
			})
			if err != nil {
				return err
			}

			if diffFlag {
				differ, err := diffutil.NewDiffer()
				if err != nil {
					return fmt.Errorf("failed to create differ: %v", err)
				}
				defer differ.Cleanup()

				if err := configset.AddObjectResultsToDiffer(res.ObjectResults, differ, configset.AddObjectResultsToDifferOptions{
					StripManagedFields: stripManagedFieldsFlag,
				}); err != nil {
					return fmt.Errorf("failed to write object results to differ: %v", err)
				}

				if err := differ.Run(diffProgram(), c.OutOrStdout(), c.ErrOrStderr()); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "If true, submit server-side request without persisting the resource.")
	cmd.Flags().BoolVar(&diffFlag, "diff", false, "If true, dry run and compares changes. Use 'KUBECTL_EXTERNAL_DIFF' to specify a custom differ, default being '"+defaultDiffProgram+"'.")
	cmd.Flags().BoolVar(&stripManagedFieldsFlag, "strip-managed-fields", false, "If true, strip managed fields when comparing changes.")

	return cmd
}
