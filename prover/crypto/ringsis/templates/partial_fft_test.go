package main

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	//go:embed testcases/partial_fft_64_3_2.txt
	case_64_3_2 string
)

func TestPartialFFT(t *testing.T) {
	str := partialFFT(64, 2, 3)
	assert.Equal(t, case_64_3_2, str)
}
