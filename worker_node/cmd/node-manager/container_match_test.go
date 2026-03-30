package main

import (
	"testing"

	"github.com/cypher0n3/cynodeai/worker_node/internal/containerps"
)

func TestContainerNameMatch(t *testing.T) {
	tests := []struct {
		ps   string
		name string
		want bool
	}{
		{
			ps:   "cynodeai-managed-pma-test\n",
			name: "cynodeai-managed-pma",
			want: false,
		},
		{
			ps:   "cynodeai-managed-pma\n",
			name: "cynodeai-managed-pma",
			want: true,
		},
		{
			ps:   "foo\ncynodeai-managed-pma\nbar\n",
			name: "cynodeai-managed-pma",
			want: true,
		},
	}
	for _, tt := range tests {
		if got := containerps.NameListed(tt.ps, tt.name); got != tt.want {
			t.Errorf("containerps.NameListed(ps=%q, name=%q) = %v, want %v", tt.ps, tt.name, got, tt.want)
		}
	}
}
