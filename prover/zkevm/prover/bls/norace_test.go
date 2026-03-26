//go:build !race

package bls

import "testing"

func skipIfRace(_ *testing.T) {}
