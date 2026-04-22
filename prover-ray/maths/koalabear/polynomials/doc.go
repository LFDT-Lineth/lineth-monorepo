// Package polynomials provides native polynomial evaluation utilities over the
// KoalaBear field and its degree-4 extension, using the union types
// [field.Vec] and [field.Gen] for type-aware dispatch.
//
// Two evaluation bases are supported:
//   - Canonical (coefficient) form: P(X) = Σᵢ p[i]·Xⁱ
//   - Lagrange (evaluation) form: P(X) given by its values P(ωⁱ) at roots of unity
package polynomials
