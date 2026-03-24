package artifacts

import (
	"net/url"
	"os"
	"path/filepath"
)

func setupRootlessPodmanHostForTests() {
	if os.Getenv("DOCKER_HOST") != "" {
		return
	}
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return
	}
	sock := filepath.Join(runtimeDir, "podman", "podman.sock")
	if _, err := os.Stat(sock); err != nil {
		return
	}
	_ = os.Setenv("DOCKER_HOST", "unix://"+sock)
}

func urlForceIPv4Localhost(raw, defaultPort string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	host := u.Hostname()
	if host == "localhost" || host == "::1" {
		port := u.Port()
		if port == "" {
			port = defaultPort
		}
		u.Host = "127.0.0.1:" + port
		return u.String()
	}
	return raw
}
