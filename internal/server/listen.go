package server

import (
	"net"
	"net/http"
	"strings"
	"time"
)

func ListenAndServe(addr string, handler http.Handler) (url string, serve func() error, closeFn func() error, err error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", nil, nil, err
	}
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	url = "http://" + ln.Addr().String()
	// Prefer 127.0.0.1 over [::] for browser friendliness when dual-stack.
	if strings.HasPrefix(ln.Addr().String(), "[::]:") {
		url = "http://127.0.0.1" + strings.TrimPrefix(ln.Addr().String(), "[::]")
	}
	serve = func() error {
		err := srv.Serve(ln)
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
	closeFn = func() error {
		return srv.Close()
	}
	return url, serve, closeFn, nil
}
