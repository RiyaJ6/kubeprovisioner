// Package netcheck provides DNS resolution and TCP/HTTP reachability checks.
// These simulate the pre-install diagnostics a platform team runs against
// customer-deployed (BYOC) clusters before declaring an install healthy.
package netcheck

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// DNSOptions configures a DNS + connectivity check.
type DNSOptions struct {
	Host    string
	Port    int
	DoHTTP  bool
	Path    string
	Timeout string
	Out     io.Writer
}

// CheckDNS resolves the host, optionally checks TCP connectivity, and optionally
// sends an HTTP GET. Results are printed to opts.Out with pass/fail prefixes.
func CheckDNS(ctx context.Context, opts DNSOptions) error {
	timeout, err := time.ParseDuration(opts.Timeout)
	if err != nil {
		timeout = 5 * time.Second
	}

	if opts.Path == "" {
		opts.Path = "/"
	}

	// DNS resolution
	var addrs []string
	resolver := &net.Resolver{}
	addrs, err = resolver.LookupHost(ctx, opts.Host)
	if err != nil {
		fmt.Fprintf(opts.Out, "✗  DNS: failed to resolve %s: %v\n", opts.Host, err)
		return fmt.Errorf("dns resolution failed: %w", err)
	}
	fmt.Fprintf(opts.Out, "✓  DNS: %s resolves to %s\n", opts.Host, addrs[0])

	// TCP check
	addr := fmt.Sprintf("%s:%d", addrs[0], opts.Port)
	dialer := &net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Fprintf(opts.Out, "✗  TCP: connect to %s failed: %v\n", addr, err)
		return fmt.Errorf("tcp connect failed: %w", err)
	}
	conn.Close()
	fmt.Fprintf(opts.Out, "✓  TCP: connect to %s succeeded (%dms)\n", addr, elapsed.Milliseconds())

	if !opts.DoHTTP {
		return nil
	}

	// HTTP check
	url := fmt.Sprintf("http://%s:%d%s", opts.Host, opts.Port, opts.Path)
	httpClient := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		fmt.Fprintf(opts.Out, "✗  HTTP: failed to build request: %v\n", err)
		return err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Fprintf(opts.Out, "✗  HTTP GET %s failed: %v\n", url, err)
		return fmt.Errorf("http check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Fprintf(opts.Out, "✓  HTTP GET %s returned %d\n", url, resp.StatusCode)
	} else {
		fmt.Fprintf(opts.Out, "✗  HTTP GET %s returned %d\n", url, resp.StatusCode)
		return fmt.Errorf("http returned non-2xx: %d", resp.StatusCode)
	}

	return nil
}
