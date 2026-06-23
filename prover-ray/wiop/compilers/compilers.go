// Package compilers groups the wiop compilation passes. Each pass — range
// check, lookup-to-log-derivative, log-derivative sum, local vanishing, and
// global quotient — lives in its own subpackage. This file exists so that
// pipeline-level integration tests can live alongside them in the same
// directory and observe the passes composed end-to-end.
package compilers
