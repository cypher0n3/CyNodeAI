package config

// CacheDir returns the default cache directory for non-secret ephemeral data (e.g. last thread id).
// If XDG_CACHE_HOME is set, uses $XDG_CACHE_HOME/cynork; otherwise ~/.cache/cynork.
func CacheDir() (string, error) {
	return resolveXDGOrHome("XDG_CACHE_HOME", ".cache")
}
