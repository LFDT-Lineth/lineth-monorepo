package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/verifierray/codegen"
)

// runVerifierAssets writes the verifier-ray assets of one prover-ray system: the generated Zig
// system (--zig) and, with --image, the proof image of an honest proof of it.
func runVerifierAssets(args []string, log io.Writer) error {
	fs := flag.NewFlagSet("prover verifier-assets", flag.ContinueOnError)
	system := fs.String("system", "", "system to generate: "+strings.Join(codegen.AssetSystems, ", ")+" (required)")
	zigPath := fs.String("zig", "", "path of the generated Zig system (required)")
	imagePath := fs.String("image", "", "path of the honest proof image; no proof is made when empty")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments %q", fs.Args())
	}
	if *system == "" {
		return fmt.Errorf("--system is required")
	}
	if *zigPath == "" {
		return fmt.Errorf("--zig is required")
	}

	assets, err := codegen.BuildVerifierAssets(*system, *imagePath != "")
	if err != nil {
		return err
	}
	if err := writeAsset(*zigPath, assets.SystemZig, log); err != nil {
		return err
	}
	if *imagePath != "" {
		return writeAsset(*imagePath, assets.ProofImage, log)
	}
	return nil
}

func writeAsset(path string, data []byte, log io.Writer) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("creating the directory of %s: %w", path, err)
	}
	// The assets are build inputs read by other tools, not secrets.
	if err := os.WriteFile(path, data, 0o644); err != nil { // #nosec G306
		return fmt.Errorf("writing %s: %w", path, err)
	}
	_, err := fmt.Fprintf(log, "wrote %s (%d bytes)\n", path, len(data))
	return err
}
