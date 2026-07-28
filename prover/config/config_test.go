package config

import (
	"os"
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

func TestPersistDerivedSRSDefaultsOff(t *testing.T) {
	assert := require.New(t)

	// writing into the SRS directory must never be something an operator gets by
	// omission: with the key absent, it stays off
	v := viper.New()
	v.SetConfigType("toml")
	assert.NoError(v.ReadConfig(strings.NewReader("assets_dir = \"/tmp/assets\"\n")))
	var cfg Config
	assert.NoError(v.Unmarshal(&cfg))
	assert.False(cfg.PersistDerivedSRS, "persist_derived_srs must default to false")

	// and it is settable for the self-hoster who wants it
	v = viper.New()
	v.SetConfigType("toml")
	assert.NoError(v.ReadConfig(strings.NewReader("persist_derived_srs = true\n")))
	var optedIn Config
	assert.NoError(v.Unmarshal(&optedIn))
	assert.True(optedIn.PersistDerivedSRS)
}

func TestShippedConfigsDoNotOptIntoSRSWrites(t *testing.T) {
	assert := require.New(t)

	// none of the checked-in configs may turn the write on: a deployment that
	// wants it has to say so itself
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
		var cfg Config
		assert.NoError(v.Unmarshal(&cfg), f.Name())
		assert.False(cfg.PersistDerivedSRS, "%s must not opt into SRS writes", f.Name())
		checked++
	}
	assert.Greater(checked, 0, "no config files were checked")
}
