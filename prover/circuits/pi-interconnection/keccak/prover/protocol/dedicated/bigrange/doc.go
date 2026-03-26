/*
Package `bigrange` provides a utility to enforce range-checks for large ranges.
The implementation is performed by decomposing the large range into a multiple
smaller range and applying a [query.Range] over them.

It is used in the [plonk.PlonkCheck] when passing the option to use external
range-checks. And more generally, it can be used for any use-case where the
range is too large for [wizard.CompiledIOP.InsertRange] to process. Otherwise,
[wizard.CompiledIOP.InsertRange] should be preferred.
*/
package bigrange
