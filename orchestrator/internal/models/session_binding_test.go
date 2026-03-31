package models

import (
	"testing"

	"github.com/google/uuid"
)

func TestSessionBinding_DeriveSessionBindingKey_SameLineageSameKey(t *testing.T) {
	u := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	s := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	th := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	lineage := SessionBindingLineage{UserID: u, SessionID: s, ThreadID: &th}
	k1 := DeriveSessionBindingKey(lineage)
	k2 := DeriveSessionBindingKey(lineage)
	if k1 != k2 {
		t.Fatalf("same lineage: got %q and %q", k1, k2)
	}
	if len(k1) != 64 {
		t.Fatalf("expected sha256 hex length 64, got %d", len(k1))
	}
}

func TestSessionBinding_DeriveSessionBindingKey_NilThreadUsesSentinel(t *testing.T) {
	u := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	s := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	withNil := DeriveSessionBindingKey(SessionBindingLineage{UserID: u, SessionID: s, ThreadID: nil})
	allZero := uuid.Nil
	withZero := DeriveSessionBindingKey(SessionBindingLineage{UserID: u, SessionID: s, ThreadID: &allZero})
	if withNil != withZero {
		t.Fatalf("nil thread should match uuid.Nil thread id: %q vs %q", withNil, withZero)
	}
}

func TestSessionBinding_DeriveSessionBindingKey_DifferentLineageComponentsDoNotCollide(t *testing.T) {
	u1 := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	u2 := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	s1 := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	s2 := uuid.MustParse("55555555-5555-4555-8555-555555555555")
	cases := []struct {
		name string
		a, b SessionBindingLineage
	}{
		{
			name: "different_users_same_session",
			a:    SessionBindingLineage{UserID: u1, SessionID: s1, ThreadID: nil},
			b:    SessionBindingLineage{UserID: u2, SessionID: s1, ThreadID: nil},
		},
		{
			name: "same_user_different_sessions",
			a:    SessionBindingLineage{UserID: u1, SessionID: s1, ThreadID: nil},
			b:    SessionBindingLineage{UserID: u1, SessionID: s2, ThreadID: nil},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k1 := DeriveSessionBindingKey(tc.a)
			k2 := DeriveSessionBindingKey(tc.b)
			if k1 == k2 {
				t.Fatalf("lineages must not collide: %q", k1)
			}
		})
	}
}

func TestSessionBinding_DeriveSessionBindingKey_DifferentThreads(t *testing.T) {
	u := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	s := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	th1 := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	th2 := uuid.MustParse("66666666-6666-4666-8666-666666666666")
	k1 := DeriveSessionBindingKey(SessionBindingLineage{UserID: u, SessionID: s, ThreadID: &th1})
	k2 := DeriveSessionBindingKey(SessionBindingLineage{UserID: u, SessionID: s, ThreadID: &th2})
	if k1 == k2 {
		t.Fatalf("different threads must not collide: %q", k1)
	}
}

func TestSessionBinding_StateConstants(t *testing.T) {
	if SessionBindingStateActive == "" || SessionBindingStateTeardownPending == "" {
		t.Fatal("binding state constants must be non-empty")
	}
	if SessionBindingStateActive == SessionBindingStateTeardownPending {
		t.Fatal("binding state constants must differ")
	}
}
