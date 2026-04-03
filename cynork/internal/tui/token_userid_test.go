package tui

import "testing"

func TestUserIDFromAccessTokenUnverified(t *testing.T) {
	// eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhMWIyYzNkNC1lNWY2LTc4OTAtYWJjZC1lZmdoaWprbG1uIn0.sig
	// {"alg":"HS256"} . {"sub":"a1b2c3d4-e5f6-7890-abcd-efghijklmn"} (payload only matters)
	const tok = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhMWIyYzNkNC1lNWY2LTc4OTAtYWJjZC1lZmdoaWprbG1uIn0.x"
	if got := userIDFromAccessTokenUnverified(tok); got != "a1b2c3d4-e5f6-7890-abcd-efghijklmn" {
		t.Fatalf("got %q", got)
	}
	if userIDFromAccessTokenUnverified("") != "" || userIDFromAccessTokenUnverified("nope") != "" {
		t.Fatal("expected empty for bad input")
	}
	if userIDFromAccessTokenUnverified("a.b.c") != "" {
		t.Fatal("invalid base64 payload should yield empty")
	}
	if userIDFromAccessTokenUnverified("eyJhbGciOiJIUzI1NiJ9.e30.sig") != "" {
		t.Fatal("missing sub should yield empty")
	}
}
