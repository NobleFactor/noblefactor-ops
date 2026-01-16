// SPDX-License-Identifier: MIT
// Copyright (c) 2025 Noble Factor. All rights reserved.

// nf-ops is the internal operations tool for DevLore.
// It handles release signing, key management, and registry maintenance.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "nf-ops",
		Short: "DevLore internal operations tool",
		Long: `nf-ops is the internal operations tool for DevLore.

It handles release signing, key management, and registry maintenance.
Most operations require human presence and hardware key (YubiKey/HSM).

See ADR-040 for the full key management protocol.`,
	}

	// Version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("nf-ops %s (%s) built %s\n", version, commit, buildDate)
		},
	})

	// Key management commands
	keyCmd := &cobra.Command{
		Use:   "key",
		Short: "Key management operations",
	}
	keyCmd.AddCommand(&cobra.Command{
		Use:   "generate",
		Short: "Generate a new signing key",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Key generation not yet implemented")
			fmt.Println("See ADR-040 for the key ceremony protocol")
		},
	})
	keyCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List managed signing keys",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Key listing not yet implemented")
		},
	})
	keyCmd.AddCommand(&cobra.Command{
		Use:   "rotate",
		Short: "Rotate a signing key with ceremony",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Key rotation not yet implemented")
			fmt.Println("This operation requires hardware key presence")
		},
	})
	rootCmd.AddCommand(keyCmd)

	// Signing commands
	signCmd := &cobra.Command{
		Use:   "sign",
		Short: "Sign artifacts",
	}
	signCmd.AddCommand(&cobra.Command{
		Use:   "pmm <path>",
		Short: "Sign a PMM with release key",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("PMM signing not yet implemented: %s\n", args[0])
		},
	})
	signCmd.AddCommand(&cobra.Command{
		Use:   "index",
		Short: "Generate and sign INDEX.yaml",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Index signing not yet implemented")
		},
	})
	signCmd.AddCommand(&cobra.Command{
		Use:   "binary <path>",
		Short: "Sign a release binary",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Binary signing not yet implemented: %s\n", args[0])
		},
	})
	rootCmd.AddCommand(signCmd)

	// Registry commands
	registryCmd := &cobra.Command{
		Use:   "registry",
		Short: "Registry maintenance operations",
	}
	registryCmd.AddCommand(&cobra.Command{
		Use:   "reindex",
		Short: "Regenerate INDEX.yaml from PMMs",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Registry reindexing not yet implemented")
		},
	})
	registryCmd.AddCommand(&cobra.Command{
		Use:   "verify",
		Short: "Verify all PMM signatures",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Signature verification not yet implemented")
		},
	})
	registryCmd.AddCommand(&cobra.Command{
		Use:   "audit",
		Short: "Generate audit trail report",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Audit report not yet implemented")
		},
	})
	rootCmd.AddCommand(registryCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
