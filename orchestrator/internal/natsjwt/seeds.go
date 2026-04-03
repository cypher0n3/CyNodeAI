package natsjwt

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nats-io/nkeys"
)

// Environment keys for the four dev nkey seeds (set all four, or none — see LoadDevSeeds).
const (
	EnvDevOperatorSeed      = "NATS_DEV_OPERATOR_SEED"
	EnvDevSystemAccountSeed = "NATS_DEV_SYSTEM_ACCOUNT_SEED"
	EnvDevAccountSeed       = "NATS_DEV_ACCOUNT_SEED"
	EnvDevSigningSeed       = "NATS_DEV_SIGNING_SEED"
	EnvDevSeedsFile         = "NATS_DEV_SEEDS_FILE"
)

// DevSeeds holds nkey seeds for local NATS dev: operator, SYS account, CYNODE account identity, CYNODE signing key.
type DevSeeds struct {
	Operator      string `json:"operator_seed"`
	SystemAccount string `json:"system_account_seed"`
	CynodeAccount string `json:"cynode_account_seed"`
	CynodeSigning string `json:"cynode_signing_seed"`
}

var (
	seedsMu      sync.Mutex
	seedsCached  *DevSeeds
	seedsLoadErr error
)

// ResetDevSeedsCache clears the in-process cache (e.g. after changing NATS_DEV_* or NATS_DEV_SEEDS_FILE in tests).
func ResetDevSeedsCache() {
	seedsMu.Lock()
	defer seedsMu.Unlock()
	seedsCached = nil
	seedsLoadErr = nil
}

// LoadDevSeeds returns dev NATS seeds from (in order): all four env vars, JSON file (NATS_DEV_SEEDS_FILE or default
// under XDG cache), or newly generated random keys written to that file.
func LoadDevSeeds() (*DevSeeds, error) {
	seedsMu.Lock()
	defer seedsMu.Unlock()
	if seedsCached != nil {
		return seedsCached, nil
	}
	if seedsLoadErr != nil {
		return nil, seedsLoadErr
	}
	s, err := loadDevSeedsUncached()
	if err != nil {
		seedsLoadErr = err
		return nil, err
	}
	seedsCached = s
	return s, nil
}

func defaultSeedsFilePath() string {
	if p := strings.TrimSpace(os.Getenv(EnvDevSeedsFile)); p != "" {
		return p
	}
	return filepath.Join(devCacheRoot(), "nats-dev-seeds.json")
}

func devCacheRoot() string {
	if d := os.Getenv("XDG_CACHE_HOME"); d != "" {
		return filepath.Join(d, "cynodeai")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "cynodeai")
	}
	return filepath.Join(home, ".cache", "cynodeai")
}

//nolint:gocognit,gocyclo // Dev seed resolution: env vars, file, or generate — kept in one function for clarity.
func loadDevSeedsUncached() (*DevSeeds, error) {
	op := strings.TrimSpace(os.Getenv(EnvDevOperatorSeed))
	sys := strings.TrimSpace(os.Getenv(EnvDevSystemAccountSeed))
	cyn := strings.TrimSpace(os.Getenv(EnvDevAccountSeed))
	sig := strings.TrimSpace(os.Getenv(EnvDevSigningSeed))
	n := 0
	if op != "" {
		n++
	}
	if sys != "" {
		n++
	}
	if cyn != "" {
		n++
	}
	if sig != "" {
		n++
	}
	if n == 4 {
		s := &DevSeeds{Operator: op, SystemAccount: sys, CynodeAccount: cyn, CynodeSigning: sig}
		if err := validateDevSeeds(s); err != nil {
			return nil, err
		}
		return s, nil
	}
	if n != 0 {
		return nil, fmt.Errorf(
			"natsjwt: set all of %s, %s, %s, %s or none (file %s or auto-generated)",
			EnvDevOperatorSeed, EnvDevSystemAccountSeed, EnvDevAccountSeed, EnvDevSigningSeed, EnvDevSeedsFile,
		)
	}

	path := defaultSeedsFilePath()
	if data, err := os.ReadFile(path); err == nil {
		var s DevSeeds
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, fmt.Errorf("natsjwt: parse %s: %w", path, err)
		}
		if err := validateDevSeeds(&s); err != nil {
			return nil, fmt.Errorf("natsjwt: invalid seeds in %s: %w", path, err)
		}
		return &s, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("natsjwt: read %s: %w", path, err)
	}

	s, err := GenerateRandomDevSeeds()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		// Another process may have created path; read it.
		if data2, err2 := os.ReadFile(path); err2 == nil {
			var s2 DevSeeds
			if err := json.Unmarshal(data2, &s2); err == nil {
				if err := validateDevSeeds(&s2); err == nil {
					return &s2, nil
				}
			}
		}
		return nil, fmt.Errorf("natsjwt: write %s: %w", path, err)
	}
	return s, nil
}

func validateDevSeeds(s *DevSeeds) error {
	if s == nil {
		return errors.New("natsjwt: nil seeds")
	}
	for label, v := range map[string]string{
		EnvDevOperatorSeed:      s.Operator,
		EnvDevSystemAccountSeed: s.SystemAccount,
		EnvDevAccountSeed:       s.CynodeAccount,
		EnvDevSigningSeed:       s.CynodeSigning,
	} {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("natsjwt: empty %s", label)
		}
		if _, err := nkeys.FromSeed([]byte(v)); err != nil {
			return fmt.Errorf("natsjwt: invalid %s: %w", label, err)
		}
	}
	return nil
}

// GenerateRandomDevSeeds creates a new random dev seed set (not persisted).
func GenerateRandomDevSeeds() (*DevSeeds, error) {
	okp, err := nkeys.CreateOperator()
	if err != nil {
		return nil, err
	}
	sysKp, err := nkeys.CreateAccount()
	if err != nil {
		return nil, err
	}
	cynKp, err := nkeys.CreateAccount()
	if err != nil {
		return nil, err
	}
	signKp, err := nkeys.CreateAccount()
	if err != nil {
		return nil, err
	}
	os_, err := okp.Seed()
	if err != nil {
		return nil, err
	}
	ss, err := sysKp.Seed()
	if err != nil {
		return nil, err
	}
	cs, err := cynKp.Seed()
	if err != nil {
		return nil, err
	}
	sg, err := signKp.Seed()
	if err != nil {
		return nil, err
	}
	return &DevSeeds{
		Operator:      string(os_),
		SystemAccount: string(ss),
		CynodeAccount: string(cs),
		CynodeSigning: string(sg),
	}, nil
}
