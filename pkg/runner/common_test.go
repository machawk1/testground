package runner

import (
	"testing"
)

func TestNextDataNetwork(t *testing.T) {
	var tests = []struct {
		lenNetworks int
		subnet      string
		gateway     string
		hasError    bool
	}{
		{0, "16.0.0.0/16", "16.0.0.1", false},
		{1, "16.1.0.0/16", "16.1.0.1", false},
		{2, "16.2.0.0/16", "16.2.0.1", false},
		{255, "16.255.0.0/16", "16.255.0.1", false},
		{256, "17.0.0.0/16", "17.0.0.1", false},
		{4095, "31.255.0.0/16", "31.255.0.1", false},
		{4096, "", "", true},
	}

	for _, tt := range tests {
		subnet, gateway, err := nextDataNetwork(tt.lenNetworks)
		if err != nil {
			if !tt.hasError {
				t.Errorf("got error but didn't expect one: %s", err)
			}
			continue
		}
		if subnet.String() != tt.subnet || gateway != tt.gateway {
			t.Errorf("got subnet %s gateway %s, want %s and %s", subnet, gateway, tt.subnet, tt.gateway)
		}
	}
}
