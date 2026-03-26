package protocol

import "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"

/*
Main object responsible for building an IOP protocol
It is exposed to the user directly, and the user should
use it to specify his protocol
*/
type Builder = wizard.Builder

/*
Function to specify the definition of an IOP
*/
type DefineFunc = wizard.DefineFunc

/*
Compile an IOP from a protocol definition
*/
var Compile = wizard.Compile

/*
User provided function that run the "top-level" protocol.
Function to be run by the prover to pass the values to the
prover (i.e) what values to commit to etc...
*/
type ProverSteps = wizard.MainProverStep

/*
Run the prover of the compiled protocol
*/
var Prover = wizard.Prove
