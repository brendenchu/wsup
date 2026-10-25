package cli

import "testing"

func TestHardeningCIFailureProbe(t *testing.T) {
	t.Fatal("intentional hardening verification failure; never merge this probe")
}
