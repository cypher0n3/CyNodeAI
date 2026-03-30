package gateway

import "testing"

func TestClientTimeout(t *testing.T) {
	c := NewClient("http://example.com")
	if c.HTTPClient.Timeout != defaultGatewayClientTimeout {
		t.Fatalf("HTTPClient.Timeout = %v, want %v", c.HTTPClient.Timeout, defaultGatewayClientTimeout)
	}
}

func TestClientRace(t *testing.T) {
	c := NewClient("http://a")
	const n = 64
	done := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			c.SetToken("tok")
			_ = c.Token()
			c.SetBaseURL("http://b")
			_ = c.BaseURL()
		}()
	}
	for i := 0; i < n; i++ {
		<-done
	}
}
