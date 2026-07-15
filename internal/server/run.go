package server

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
)

// RunOptions configures a short-lived workbench HTTP process (no daemon).
type RunOptions struct {
	Addr        string
	Handler     http.Handler
	IndexPath   string // printed for the operator
	OpenBrowser bool
}

// Run listens, optionally opens the browser, blocks until SIGINT/SIGTERM or server error,
// then closes the listener. Process lifetime is tied to this call.
func Run(opts RunOptions) error {
	url, serve, closeFn, err := ListenAndServe(opts.Addr, opts.Handler)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer closeFn()

	fmt.Printf("分类工作台  %s\n", url)
	if opts.IndexPath != "" {
		fmt.Printf("INDEX       %s\n", opts.IndexPath)
	}
	fmt.Fprintf(os.Stderr, "Press Ctrl+C to stop (process exits with the command; no daemon).\n")

	if opts.OpenBrowser {
		if err := openBrowser(url); err != nil {
			fmt.Fprintf(os.Stderr, "skills-manage: open browser: %v (open %s manually)\n", err, url)
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	errCh := make(chan error, 1)
	go func() { errCh <- serve() }()

	select {
	case sig := <-sigCh:
		fmt.Fprintf(os.Stderr, "\nstopping (%v)…\n", sig)
		_ = closeFn()
		<-errCh
		return nil
	case err := <-errCh:
		return err
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported OS %q", runtime.GOOS)
	}
	return cmd.Start()
}
