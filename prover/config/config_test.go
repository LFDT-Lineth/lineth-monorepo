package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestEnvironment(t *testing.T) {
	assert := require.New(t)

	// parse each config file and ensure environment is well set.
	// we also ensure we can parse the config file without error.
	// look for all files with config-XXX.toml in current dir and capture XXX with a regexp.

	// For example for these file names, the regexp captures the following:
	// config-integration-development.toml 	--> integration-development
	// config-integration-full.toml 		--> integration-full
	re := regexp.MustCompile(`config-(.*)\.toml`)

	// get all files in current dir
	files, err := os.ReadDir(".")
	assert.NoError(err)

	count := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		matches := re.FindStringSubmatch(file.Name())
		if len(matches) == 0 {
			continue
		}
		count++
		t.Logf("loading config file %s - %s", file.Name(), matches[1])

		t.Run(matches[1], func(t *testing.T) {
			viper.Set("assets_dir", "../prover-assets")
			config, err := NewConfigFromFile(file.Name())
			assert.NoError(err, "when processing %s", file.Name())

			// take the first word of both the match and the environment
			// sepolia-full -> sepolia
			var (
				matchFirst = strings.Split(matches[1], "-")[0]
				envFirst   = strings.Split(config.Environment, "-")[0]
			)

			// check that the environment is set
			assert.Equal(matchFirst, envFirst)
		})
	}

	assert.NotEqual(0, count, "no config file found")
}

func TestMustFindModuleLimitsReturnsLongestMatch(t *testing.T) {
	tl := GetTestTracesLimits()
	// For every module in the config, mustFindModuleLimits must return that
	// exact module's limits (not a shorter prefix match like "").
	for _, m := range tl.Modules {
		if m.Module == "" {
			continue
		}
		got := tl.mustFindModuleLimits(m.Module)
		if got.Module != m.Module {
			t.Errorf("mustFindModuleLimits(%q) returned %q, want exact match", m.Module, got.Module)
		}
		if got.Limit != m.Limit {
			t.Errorf("mustFindModuleLimits(%q) returned limit %d, want %d", m.Module, got.Limit, m.Limit)
		}
	}
}

func TestPersistDerivedSRSDefaultsOn(t *testing.T) {
	assert := require.New(t)

	// a missing lagrange dump is otherwise re-derived silently for hours on
	// every prover start, so persistence at setup is what an operator gets by
	// omission; the default is applied by the real loading path

	// the smallest config the unchecked loading path accepts: the layer2
	// addresses are parsed unconditionally, even without validation
	minimal := `assets_dir = "/tmp/assets"
[layer2]
message_service_contract = "0x0000000000000000000000000000000000000000"
coin_base = "0x0000000000000000000000000000000000000000"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config-test.toml")
	assert.NoError(os.WriteFile(path, []byte(minimal), 0o600))
	cfg, err := NewConfigFromFileUnchecked(path)
	assert.NoError(err)
	assert.True(cfg.PersistDerivedSRS, "persist_derived_srs must default to true")

	// and the immutable-SRS-directory deployment can still opt out (the key is
	// top-level, so it must precede the [layer2] table)
	assert.NoError(os.WriteFile(path, []byte("persist_derived_srs = false\n"+minimal), 0o600))
	optedOut, err := NewConfigFromFileUnchecked(path)
	assert.NoError(err)
	assert.False(optedOut.PersistDerivedSRS)
}

func TestShippedConfigsDoNotOptOutOfSRSWrites(t *testing.T) {
	assert := require.New(t)

	// persistence at setup is the default cure for silent hours-long
	// re-derivation, so a checked-in config must not quietly reintroduce it;
	// opting out is a per-deployment decision made in that deployment's own
	// config, not in the repo's
	files, err := os.ReadDir(".")
	assert.NoError(err)
	checked := 0
	for _, f := range files {
		if !strings.HasPrefix(f.Name(), "config-") || !strings.HasSuffix(f.Name(), ".toml") {
			continue
		}
		v := viper.New()
		v.SetConfigFile(f.Name())
		assert.NoError(v.ReadInConfig(), f.Name())
		if v.IsSet("persist_derived_srs") {
			assert.True(v.GetBool("persist_derived_srs"), "%s must not opt out of SRS persistence", f.Name())
		}
		checked++
	}
	assert.Greater(checked, 0, "no config files were checked")
}
