package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/wxdao/configset/pkg/configset"
	"github.com/wxdao/configset/pkg/diffutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
)

func NewApplyCmd(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	fileNameFlags := genericclioptions.FileNameFlags{
		Usage:     "identifying the resource.",
		Filenames: &[]string{},
		Recursive: func(b bool) *bool { return &b }(false),
		Kustomize: func(s string) *string { return &s }(""),
	}
	forceConflictsFlag := false
	dryRunFlag := false
	diffFlag := false

	cmd := &cobra.Command{
		Use:          "apply",
		Short:        "Apply a config set to Kubernetes.",
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("must specify a config set name")
			}
			setName := args[0]

			if diffFlag {
				dryRunFlag = true
			}

			restConfig, err := configFlags.ToRESTConfig()
			if err != nil {
				return fmt.Errorf("failed to get rest config: %v", err)
			}

			namespace, enforceNamespace, err := configFlags.ToRawKubeConfigLoader().Namespace()
			if err != nil {
				return fmt.Errorf("failed to get namespace: %v", err)
			}

			fnOpt := fileNameFlags.ToOptions()
			if err := fnOpt.RequireFilenameOrKustomize(); err != nil {
				return err
			}
			builder := resource.NewLocalBuilder().
				Unstructured().
				Flatten().
				NamespaceParam(namespace).DefaultNamespace().
				FilenameParam(enforceNamespace, &fnOpt)

			result := builder.Do()
			infos, err := result.Infos()
			if err != nil {
				return fmt.Errorf("failed to get resource infos: %v", err)
			}
			objs := make([]configset.Object, 0, len(infos))
			for _, info := range infos {
				obj := info.Object.(*unstructured.Unstructured)
				if obj.GetKind() != "Namespace" && obj.GetNamespace() == "" {
					obj.SetNamespace(namespace)
					info.Namespace = namespace
				}
				objs = append(objs, obj)
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

			res, err := cli.Apply(c.Context(), setName, objs, configset.ApplyOptions{
				DryRun:         dryRunFlag,
				ForceConflicts: forceConflictsFlag,
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
			if err != nil {
				return err
			}

			if diffFlag {
				differ, err := diffutil.NewDiffer()
				if err != nil {
					return fmt.Errorf("failed to create differ: %v", err)
				}
				defer differ.Cleanup()

				if err := configset.WriteObjectResultsToDiffer(res.ObjectResults, differ, ""); err != nil {
					return fmt.Errorf("failed to write object results to differ: %v", err)
				}

				if err := differ.Run(diffProgram(), c.OutOrStdout(), c.ErrOrStderr()); err != nil {
					return err
				}
			}

			return nil
		},
	}

	fileNameFlags.AddFlags(cmd.Flags())
	cmd.Flags().BoolVar(&forceConflictsFlag, "force-conflicts", false, "If true, apply will force the changes against conflicts.")
	cmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "If true, submit server-side request without persisting the resource.")
	cmd.Flags().BoolVar(&diffFlag, "diff", false, "If true, dry run and compares changes. Use 'KUBECTL_EXTERNAL_DIFF' to specify a custom differ, default being '"+defaultDiffProgram+"'.")

	return cmd
}
