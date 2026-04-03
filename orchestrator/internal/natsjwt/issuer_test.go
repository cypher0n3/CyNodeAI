package natsjwt

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	jwt "github.com/nats-io/jwt/v2"
)

func mustDevSeeds(t *testing.T) *DevSeeds {
	t.Helper()
	s, err := LoadDevSeeds()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewIssuer_EmptySeeds(t *testing.T) {
	t.Parallel()
	ds := mustDevSeeds(t)
	if _, err := NewIssuer("", ds.CynodeSigning); err == nil {
		t.Fatal("expected error")
	}
	if _, err := NewIssuer(ds.CynodeAccount, ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewIssuer_InvalidAccountSeed(t *testing.T) {
	t.Parallel()
	ds := mustDevSeeds(t)
	if _, err := NewIssuer("not-a-seed", ds.CynodeSigning); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewIssuer_InvalidSigningSeed(t *testing.T) {
	t.Parallel()
	ds := mustDevSeeds(t)
	if _, err := NewIssuer(ds.CynodeAccount, "not-a-seed"); err == nil {
		t.Fatal("expected error")
	}
}

func TestRevokeJTI_Empty(t *testing.T) {
	t.Parallel()
	iss, err := NewDevIssuer()
	if err != nil {
		t.Fatal(err)
	}
	iss.RevokeJTI("")
	if iss.Revoked("") {
		t.Fatal("empty jti should not revoke")
	}
}

func TestRevoked_EmptyJTI(t *testing.T) {
	t.Parallel()
	iss, err := NewDevIssuer()
	if err != nil {
		t.Fatal(err)
	}
	if iss.Revoked("") {
		t.Fatal("expected false")
	}
}

func TestExtractJTI_InvalidToken(t *testing.T) {
	t.Parallel()
	if _, err := ExtractJTI("not-a-jwt"); err == nil {
		t.Fatal("expected error")
	}
}

func TestRevokeSessionNatsJWT_InvalidToken(t *testing.T) {
	t.Parallel()
	iss, err := NewDevIssuer()
	if err != nil {
		t.Fatal(err)
	}
	if err := iss.RevokeSessionNatsJWT("not-a-jwt"); err == nil {
		t.Fatal("expected error")
	}
}

func TestNatsJwt_SessionJWTPermissionsAndExpiry(t *testing.T) {
	t.Parallel()
	iss, err := NewDevIssuer()
	if err != nil {
		t.Fatal(err)
	}
	sid := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	exp := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	tok, err := iss.SessionJWT(DefaultTenantID, sid, exp)
	if err != nil {
		t.Fatal(err)
	}
	uc, err := jwt.DecodeUserClaims(tok)
	if err != nil {
		t.Fatal(err)
	}
	if uc.Expires != exp.Unix() {
		t.Fatalf("expires: got %d want %d", uc.Expires, exp.Unix())
	}
	wantSubs := []string{
		"cynode.session.activity.default." + sid.String(),
		"cynode.chat.request." + sid.String(),
	}
	for _, w := range wantSubs {
		if !uc.Pub.Allow.Contains(w) {
			t.Fatalf("missing pub allow %q: %#v", w, uc.Pub.Allow)
		}
	}
	wantActivitySub := "cynode.session.activity.default." + sid.String()
	if !uc.Sub.Allow.Contains(wantActivitySub) {
		t.Fatalf("missing sub allow for session activity %q: %#v", wantActivitySub, uc.Sub.Allow)
	}
	found := false
	prefix := "cynode.chat.stream." + sid.String()
	for _, s := range uc.Sub.Allow {
		if strings.HasPrefix(s, prefix) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing stream sub prefix %q: %#v", prefix, uc.Sub.Allow)
	}
}

func TestNatsJwt_NodeJWTPermissions(t *testing.T) {
	t.Parallel()
	iss, err := NewDevIssuer()
	if err != nil {
		t.Fatal(err)
	}
	nid := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	exp := time.Now().UTC().Add(24 * time.Hour)
	tok, err := iss.NodeJWT(DefaultTenantID, nid, exp)
	if err != nil {
		t.Fatal(err)
	}
	uc, err := jwt.DecodeUserClaims(tok)
	if err != nil {
		t.Fatal(err)
	}
	if !uc.IsBearerToken() {
		t.Fatal("expected bearer token for node jwt")
	}
	if !uc.Pub.Allow.Contains("cynode.session.activity.>") {
		t.Fatalf("expected session.activity publish: %#v", uc.Pub.Allow)
	}
	want := "cynode.node.config_changed.default." + nid.String()
	if !uc.Sub.Allow.Contains(want) {
		t.Fatalf("missing sub %q: %#v", want, uc.Sub.Allow)
	}
}

func TestNatsJwt_RevokeSessionNatsJWT(t *testing.T) {
	t.Parallel()
	iss, err := NewDevIssuer()
	if err != nil {
		t.Fatal(err)
	}
	sid := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	tok, err := iss.SessionJWT(DefaultTenantID, sid, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	jti, err := ExtractJTI(tok)
	if err != nil || jti == "" {
		t.Fatalf("jti: %v %q", err, jti)
	}
	if iss.Revoked(jti) {
		t.Fatal("unexpected revoked before revoke")
	}
	if err := iss.RevokeSessionNatsJWT(tok); err != nil {
		t.Fatal(err)
	}
	if !iss.Revoked(jti) {
		t.Fatal("expected revoked after RevokeSessionNatsJWT")
	}
}

func TestJWT_DefaultTenantWhenTenantIDEmpty(t *testing.T) {
	t.Parallel()
	iss, err := NewDevIssuer()
	if err != nil {
		t.Fatal(err)
	}
	exp := time.Now().Add(time.Hour)
	sid := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	sTok, err := iss.SessionJWT("", sid, exp)
	if err != nil {
		t.Fatal(err)
	}
	sClaims, err := jwt.DecodeUserClaims(sTok)
	if err != nil {
		t.Fatal(err)
	}
	if want := "cynode.session.activity.default." + sid.String(); !sClaims.Pub.Allow.Contains(want) {
		t.Fatalf("session missing %q: %#v", want, sClaims.Pub.Allow)
	}
	nid := uuid.MustParse("55555555-5555-4555-8555-555555555555")
	nTok, err := iss.NodeJWT("", nid, exp)
	if err != nil {
		t.Fatal(err)
	}
	nClaims, err := jwt.DecodeUserClaims(nTok)
	if err != nil {
		t.Fatal(err)
	}
	if want := "cynode.node.config_changed.default." + nid.String(); !nClaims.Pub.Allow.Contains(want) {
		t.Fatalf("node missing pub %q: %#v", want, nClaims.Pub.Allow)
	}
}

func TestSessionJWT_CustomTenantInSubjects(t *testing.T) {
	t.Parallel()
	iss, err := NewDevIssuer()
	if err != nil {
		t.Fatal(err)
	}
	sid := uuid.MustParse("66666666-6666-4666-8666-666666666666")
	exp := time.Now().Add(time.Hour)
	tok, err := iss.SessionJWT("acme", sid, exp)
	if err != nil {
		t.Fatal(err)
	}
	uc, err := jwt.DecodeUserClaims(tok)
	if err != nil {
		t.Fatal(err)
	}
	want := "cynode.session.activity.acme." + sid.String()
	if !uc.Pub.Allow.Contains(want) {
		t.Fatalf("missing %q in %#v", want, uc.Pub.Allow)
	}
	if !uc.Sub.Allow.Contains(want) {
		t.Fatalf("missing sub %q in %#v", want, uc.Sub.Allow)
	}
}

func TestNodeJWT_CustomTenantInSubjects(t *testing.T) {
	t.Parallel()
	iss, err := NewDevIssuer()
	if err != nil {
		t.Fatal(err)
	}
	nid := uuid.MustParse("77777777-7777-4777-8777-777777777777")
	tok, err := iss.NodeJWT("acme", nid, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	uc, err := jwt.DecodeUserClaims(tok)
	if err != nil {
		t.Fatal(err)
	}
	want := "cynode.node.config_changed.acme." + nid.String()
	if !uc.Sub.Allow.Contains(want) || !uc.Pub.Allow.Contains(want) {
		t.Fatalf("missing tenant-scoped subjects: sub=%#v pub=%#v", uc.Sub.Allow, uc.Pub.Allow)
	}
}

func TestControlPlaneServiceJWT_And_GatewaySessionPublisherJWT(t *testing.T) {
	t.Parallel()
	iss, err := NewDevIssuer()
	if err != nil {
		t.Fatal(err)
	}
	exp := time.Now().Add(time.Hour)
	cp, err := iss.ControlPlaneServiceJWT(exp)
	if err != nil {
		t.Fatal(err)
	}
	gw, err := iss.GatewaySessionPublisherJWT(exp)
	if err != nil {
		t.Fatal(err)
	}
	cpClaims, err := jwt.DecodeUserClaims(cp)
	if err != nil {
		t.Fatal(err)
	}
	if !cpClaims.Sub.Allow.Contains("cynode.session.>") {
		t.Fatalf("cp sub: %#v", cpClaims.Sub.Allow)
	}
	if !cpClaims.Pub.Allow.Contains("cynode.node.config_changed.>") {
		t.Fatalf("cp pub: %#v", cpClaims.Pub.Allow)
	}
	if !cpClaims.Pub.Allow.Contains("$JS.API.>") {
		t.Fatalf("cp jetstream api pub: %#v", cpClaims.Pub.Allow)
	}
	if !cpClaims.Sub.Allow.Contains("_INBOX.>") {
		t.Fatalf("cp inbox sub: %#v", cpClaims.Sub.Allow)
	}
	gwClaims, err := jwt.DecodeUserClaims(gw)
	if err != nil {
		t.Fatal(err)
	}
	if !gwClaims.Pub.Allow.Contains("cynode.session.attached.>") {
		t.Fatalf("gw pub: %#v", gwClaims.Pub.Allow)
	}
}
