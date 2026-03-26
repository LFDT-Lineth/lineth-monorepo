package utils

import (
	"golang.org/x/exp/constraints"
)

type Range struct {
	Min, Max int
}

func Max[T constraints.Ordered](s ...T) T {
	if len(s) == 0 {
		panic("Got an empty list of arguments")
	}
	m := s[0]
	for _, v := range s {
		if m < v {
			m = v
		}
	}
	return m
}

func Min[T constraints.Ordered](s ...T) T {
	if len(s) == 0 {
		panic("Got an empty list of arguments")
	}
	m := s[0]
	for _, v := range s {
		if m > v {
			m = v
		}
	}
	return m
}
