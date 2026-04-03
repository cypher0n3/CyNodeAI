package natsjwt

import (
	"os"
	"path/filepath"
	"testing"

	jwt "github.com/nats-io/jwt/v2"
)

func TestWriteDevJWTBundle_WritesOperatorAndAccounts(t *testing.T) {
	dir := t.TempDir()
	if err := WriteDevJWTBundle(dir); err != nil {
		t.Fatal(err)
	}
	seeds, err := LoadDevSeeds()
	if err != nil {
		t.Fatal(err)
	}
	sysPub, err := accountPubFromSeed(seeds.SystemAccount)
	if err != nil {
		t.Fatal(err)
	}
	cynPub, err := accountPubFromSeed(seeds.CynodeAccount)
	if err != nil {
		t.Fatal(err)
	}
	opRaw, err := os.ReadFile(filepath.Join(dir, "operator.jwt"))
	if err != nil {
		t.Fatal(err)
	}
	oc, err := jwt.DecodeOperatorClaims(string(opRaw))
	if err != nil {
		t.Fatal(err)
	}
	if oc.SystemAccount != sysPub {
		t.Fatalf("system account: got %q want %q", oc.SystemAccount, sysPub)
	}
	for _, pub := range []string{sysPub, cynPub} {
		p := filepath.Join(dir, "accounts", pub+".jwt")
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		ac, err := jwt.DecodeAccountClaims(string(raw))
		if err != nil {
			t.Fatalf("%s: %v", pub, err)
		}
		if ac.Subject != pub {
			t.Fatalf("subject: got %q want %q", ac.Subject, pub)
		}
	}
}

func TestDefaultDevJWTBundleDir_nonEmpty(t *testing.T) {
	t.Parallel()
	d := DefaultDevJWTBundleDir()
	if d == "" {
		t.Fatal("empty dir")
	}
}
