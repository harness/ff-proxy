package main

import (
	"flag"
	"os"
	"testing"
)

func TestParseFlagsProxyKey(t *testing.T) {
	testCases := []struct {
		name string
		key  string
		want string
	}{
		{name: "unchanged", key: "Example-Proxy-Key", want: "Example-Proxy-Key"},
		{name: "leading spaces", key: "  example-key", want: "example-key"},
		{name: "trailing spaces", key: "example-key  ", want: "example-key"},
		{name: "tabs and newlines", key: "\t\r\n example-key \r\n\t", want: "example-key"},
		{name: "unicode whitespace", key: "\u00a0\u2003example-key\u2003\u00a0", want: "example-key"},
		{name: "empty", key: "", want: ""},
		{name: "whitespace only", key: " \t\r\n\u00a0", want: ""},
		{name: "internal space preserved", key: " example key ", want: "example key"},
		{name: "internal newline preserved", key: " example\nkey ", want: "example\nkey"},
		{name: "non-whitespace control preserved", key: "\x01example-key\x01", want: "\x01example-key\x01"},
		{name: "zero width space preserved", key: "\u200bexample-key\u200b", want: "\u200bexample-key\u200b"},
	}

	for _, source := range []string{"flag", "environment"} {
		t.Run(source, func(t *testing.T) {
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					originalArgs, originalFlags, originalKey := os.Args, flag.CommandLine, proxyKey
					t.Cleanup(func() {
						os.Args, flag.CommandLine, proxyKey = originalArgs, originalFlags, originalKey
					})

					flag.CommandLine = flag.NewFlagSet("ff-proxy", flag.ExitOnError)
					flag.StringVar(&proxyKey, proxyKeyFlag, "", "proxy key")
					os.Args = []string{"ff-proxy"}
					if source == "environment" {
						if tc.key != "" {
							// Non-empty environment values retain precedence over CLI flags.
							os.Args = append(os.Args, "--"+proxyKeyFlag+"=overridden-key")
						}
						t.Setenv(proxyKeyEnv, tc.key)
						loadFlagsFromEnv(map[string]string{proxyKeyEnv: proxyKeyFlag})
					} else {
						os.Args = append(os.Args, "--"+proxyKeyFlag+"="+tc.key)
					}

					parseFlags()

					if proxyKey != tc.want {
						t.Errorf("parsed proxy key = %q, want %q", proxyKey, tc.want)
					}
				})
			}
		})
	}
}
