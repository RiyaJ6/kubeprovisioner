package main

import (
	"fmt"
	"os"

	"github.com/RiyaJ6/kubeprovisioner/internal/diagnose"
	"github.com/RiyaJ6/kubeprovisioner/internal/kube"
	"github.com/RiyaJ6/kubeprovisioner/internal/netcheck"
	"github.com/RiyaJ6/kubeprovisioner/internal/report"
	"github.com/spf13/cobra"
)

var (
	kubeconfig string
	context    string
)

func main() {
	root := &cobra.Command{
		Use:   "kp",
		Short: "kubeprovisioner — cluster inspection and diagnostics CLI",
		Long: `kp inspects Kubernetes cluster state and surfaces common issues
before they become incidents. Designed for platform teams managing
SaaS and BYOC customer deployments.`,
	}

	root.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig (default: $KUBECONFIG or ~/.kube/config)")
	root.PersistentFlags().StringVar(&context, "context", "", "kubeconfig context to use")

	root.AddCommand(
		statusCmd(),
		diagnoseCmd(),
		reportCmd(),
		checkCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print node and pod health across the cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := kube.NewClient(kubeconfig, context)
			if err != nil {
				return fmt.Errorf("build kube client: %w", err)
			}
			return kube.PrintStatus(cmd.Context(), client, cmd.OutOrStdout())
		},
	}
}

func diagnoseCmd() *cobra.Command {
	var restartThreshold int
	var namespace string

	cmd := &cobra.Command{
		Use:   "diagnose",
		Short: "Surface pods with high restarts, OOMKills, or pending state",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := kube.NewClient(kubeconfig, context)
			if err != nil {
				return fmt.Errorf("build kube client: %w", err)
			}
			return diagnose.Run(cmd.Context(), client, diagnose.Options{
				RestartThreshold: restartThreshold,
				Namespace:        namespace,
				Out:              cmd.OutOrStdout(),
			})
		},
	}

	cmd.Flags().IntVar(&restartThreshold, "restarts", 5, "minimum restart count to flag")
	cmd.Flags().StringVar(&namespace, "namespace", "", "filter to namespace (default: all)")
	return cmd
}

func reportCmd() *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Emit a structured JSON report of cluster state",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := kube.NewClient(kubeconfig, context)
			if err != nil {
				return fmt.Errorf("build kube client: %w", err)
			}
			return report.Generate(cmd.Context(), client, outputFile, cmd.OutOrStdout())
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "write JSON to file (default: stdout)")
	return cmd
}

func checkCmd() *cobra.Command {
	check := &cobra.Command{
		Use:   "check",
		Short: "Network reachability checks",
	}
	check.AddCommand(dnsCmd())
	return check
}

func dnsCmd() *cobra.Command {
	var host string
	var port int
	var doHTTP bool
	var path string
	var timeout string

	cmd := &cobra.Command{
		Use:   "dns",
		Short: "Resolve a hostname and check TCP/HTTP reachability",
		RunE: func(cmd *cobra.Command, args []string) error {
			if host == "" {
				return fmt.Errorf("--host is required")
			}
			return netcheck.CheckDNS(cmd.Context(), netcheck.DNSOptions{
				Host:    host,
				Port:    port,
				DoHTTP:  doHTTP,
				Path:    path,
				Timeout: timeout,
				Out:     cmd.OutOrStdout(),
			})
		},
	}

	cmd.Flags().StringVar(&host, "host", "", "hostname to resolve (required)")
	cmd.Flags().IntVar(&port, "port", 80, "TCP port to check")
	cmd.Flags().BoolVar(&doHTTP, "http", false, "also send an HTTP GET request")
	cmd.Flags().StringVar(&path, "path", "/", "HTTP path to check")
	cmd.Flags().StringVar(&timeout, "timeout", "5s", "timeout per check")
	return cmd
}
