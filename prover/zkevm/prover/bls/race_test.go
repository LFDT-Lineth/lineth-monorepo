//go:build race

package bls

import "testing"

func skipIfRace(t *testing.T) {
	t.Skip("skipping test in race mode")
}
