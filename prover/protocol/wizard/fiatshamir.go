package wizard

import (
	"crypto/sha256"
	"io"
)

// CompiledIOPSerializer is a function capable of serializing a Compiled-IOP
type CompiledIOPSerializer = func(comp *CompiledIOP) ([]byte, error)

// VersionMetadata collects generic information to use to bootstrap the FS
// state of the already CompiledIOP.
type VersionMetadata struct {
	// Title is a generic name that can be used to identify the wizard
	Title string
	// Version number is a version string
	Version string
}

// BootstrapFiatShamir hashes the description of the struct to bootstrap the
// initial Fiat-Shamir state.
func (comp *CompiledIOP) BootstrapFiatShamir(vm VersionMetadata, ser CompiledIOPSerializer) *CompiledIOP {

	hasher := sha256.New()

	io.WriteString(hasher, vm.Title)
	io.WriteString(hasher, vm.Version)

	// compBlob, err := ser(comp)
	// if err != nil {
	// 	utils.Panic("Could not serialize the compiled IOP to bootstrap the FS state: %v", err)
	// }

	// hasher.Write(compBlob)
	digest := hasher.Sum(nil)
	digest[0] = 0 // This is to prevent potential errors due to overflowing the field
	comp.FiatShamirSetup.SetBytes(digest)

	return comp
}
