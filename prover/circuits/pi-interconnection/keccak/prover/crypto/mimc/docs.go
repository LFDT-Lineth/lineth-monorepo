// mimc wraps the [github.com/consensys/gnark-crypto/ecc/bls12-377/fr/mimc] and
// provides low-level utility methods to use the block compression of MiMC
// directly. This is used in several occasions in the wizard package to
// implement and test the arithmetization of the MiMC block compression function
// and to have interfaces that are friendly to field elements.
package mimc
