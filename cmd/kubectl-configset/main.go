package main

import (
	"os"

	"github.com/spf13/pflag"
	"github.com/wxdao/configset/pkg/cmd"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func main() {
	pflag.CommandLine = pflag.NewFlagSet("kubectl-configset", pflag.ExitOnError)

	if err := cmd.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
