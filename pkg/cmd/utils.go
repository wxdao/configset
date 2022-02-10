package cmd

import (
	"os"
)

func envOrDefault(env, def string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	return def
}

const defaultDiffProgram = "diff -N -u"

func diffProgram() string {
	return envOrDefault("KUBECTL_EXTERNAL_DIFF", defaultDiffProgram)
}
