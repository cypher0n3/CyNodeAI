// Dev NATS JWT bundle for local docker-compose (operator + SYS + CYNODE accounts).
// JWT files are written to the host cache by setup-dev, not stored in the repo.
package natsjwt

import (
	"fmt"
	"os"
	"path/filepath"

	jwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

// DefaultDevJWTBundleDir returns $XDG_CACHE_HOME/cynodeai/nats-dev-jwt (or ~/.cache/...).
func DefaultDevJWTBundleDir() string {
	if d := os.Getenv("XDG_CACHE_HOME"); d != "" {
		return filepath.Join(d, "cynodeai", "nats-dev-jwt")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "cynodeai-nats-dev-jwt")
	}
	return filepath.Join(home, ".cache", "cynodeai", "nats-dev-jwt")
}

func accountPubFromSeed(seed string) (string, error) {
	kp, err := nkeys.FromSeed([]byte(seed))
	if err != nil {
		return "", err
	}
	return kp.PublicKey()
}

// WriteDevJWTBundle writes operator.jwt and accounts/<pub>.jwt under dir (creates accounts/).
func WriteDevJWTBundle(dir string) error {
	if dir == "" {
		return fmt.Errorf("natsjwt: empty bundle dir")
	}
	seeds, err := LoadDevSeeds()
	if err != nil {
		return err
	}
	okp, err := nkeys.FromSeed([]byte(seeds.Operator))
	if err != nil {
		return fmt.Errorf("natsjwt: operator seed: %w", err)
	}
	opPub, err := okp.PublicKey()
	if err != nil {
		return err
	}

	sysPub, err := accountPubFromSeed(seeds.SystemAccount)
	if err != nil {
		return fmt.Errorf("natsjwt: system account seed: %w", err)
	}
	cynPub, err := accountPubFromSeed(seeds.CynodeAccount)
	if err != nil {
		return fmt.Errorf("natsjwt: cynode account seed: %w", err)
	}
	sigPub, err := accountPubFromSeed(seeds.CynodeSigning)
	if err != nil {
		return fmt.Errorf("natsjwt: signing seed: %w", err)
	}

	oc := jwt.NewOperatorClaims(opPub)
	oc.Name = "CYNODEAI-DEV"
	oc.SystemAccount = sysPub
	opJWT, err := oc.Encode(okp)
	if err != nil {
		return fmt.Errorf("natsjwt: encode operator: %w", err)
	}

	sysAC := jwt.NewAccountClaims(sysPub)
	sysAC.Name = "SYS"
	sysJWT, err := sysAC.Encode(okp)
	if err != nil {
		return fmt.Errorf("natsjwt: encode SYS account: %w", err)
	}

	cynAC := jwt.NewAccountClaims(cynPub)
	cynAC.Name = "CYNODE"
	cynAC.SigningKeys.Add(sigPub)
	cynAC.Limits.JetStreamLimits = jwt.JetStreamLimits{
		MemoryStorage: jwt.NoLimit,
		DiskStorage:   jwt.NoLimit,
		Streams:       jwt.NoLimit,
		Consumer:      jwt.NoLimit,
	}
	cynJWT, err := cynAC.Encode(okp)
	if err != nil {
		return fmt.Errorf("natsjwt: encode CYNODE account: %w", err)
	}

	accDir := filepath.Join(dir, "accounts")
	if err := os.MkdirAll(accDir, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "operator.jwt"), []byte(opJWT), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(accDir, sysPub+".jwt"), []byte(sysJWT), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(accDir, cynPub+".jwt"), []byte(cynJWT), 0o600); err != nil {
		return err
	}
	return nil
}
