package controller

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"syscall"
	"testing"
	"text/template"
	"time"

	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
)

func TestRunCommand(t *testing.T) {

	// Bootstrap the test directory
	confM, confL := setupFsTest(t)

	var (
		eFrom   string = confM.Execution.DirFrom()
		cFrom   string = confM.BlobDecompression.DirFrom()
		aFrom   string = confM.Aggregation.DirFrom()
		exit0   int    = 0
		exit2   int    = 2
		exit10  int    = 10
		exit12  int    = 12
		exit77  int    = 77
		exit137 int    = 137
	)

	// Populate the filesystem with job files

	// execution
	createTestInputFile(eFrom, 0, 1, execJob, exit0)
	createTestInputFile(eFrom, 1, 2, execJob, exit12, forLarge)
	createTestInputFile(eFrom, 2, 3, execJob, exit77)
	createTestInputFile(eFrom, 3, 4, execJob, exit77, forLarge)
	createTestInputFile(eFrom, 4, 5, execJob, exit137)
	createTestInputFile(eFrom, 5, 6, execJob, exit137, forLarge)
	createTestInputFile(eFrom, 6, 7, execJob, exit2)
	createTestInputFile(eFrom, 7, 8, execJob, exit2)
	createTestInputFile(eFrom, 8, 9, execJob, exit10)
	createTestInputFile(eFrom, 9, 10, execJob, exit12)

	// compression
	createTestInputFile(cFrom, 0, 2, compressionJob, exit0)
	createTestInputFile(cFrom, 2, 4, compressionJob, exit2)
	createTestInputFile(cFrom, 4, 6, compressionJob, exit77)
	createTestInputFile(cFrom, 6, 8, compressionJob, exit137)

	// aggregation
	createTestInputFile(aFrom, 0, 2, aggregationJob, exit0)
	createTestInputFile(aFrom, 2, 4, aggregationJob, exit2)
	createTestInputFile(aFrom, 4, 6, aggregationJob, exit77)
	createTestInputFile(aFrom, 6, 8, aggregationJob, exit137)

	ctxM, stopM := context.WithCancel(context.Background())
	ctxL, stopL := context.WithCancel(context.Background())

	go runController(ctxM, confM)
	go runController(ctxL, confL)

	// Give one sec, for the test to complete
	<-time.After(4 * time.Second)

	// Shutdown the two concurrent controllers
	stopM()
	stopL()

	// After this test the test directory should have the following structure
	expectedStructure := []struct {
		Path    string
		Entries []string
	}{
		{
			Path:    confM.Execution.DirFrom(),
			Entries: []string{}, // all files should be processed
		},
		{
			Path: confM.Execution.DirDone(),
			Entries: []string{
				"0-1-etv0.1.2-stv1.2.3-getZkProof.json.success",
				"1-2-etv0.1.2-stv1.2.3-getZkProof.json.large.success",
				"2-3-etv0.1.2-stv1.2.3-getZkProof.json.failure.code_67",
				"3-4-etv0.1.2-stv1.2.3-getZkProof.json.large.failure.code_65",
				"4-5-etv0.1.2-stv1.2.3-getZkProof.json.large.failure.code_125",
				"5-6-etv0.1.2-stv1.2.3-getZkProof.json.large.failure.code_125",
				"6-7-etv0.1.2-stv1.2.3-getZkProof.json.failure.code_2",
				"7-8-etv0.1.2-stv1.2.3-getZkProof.json.failure.code_2",
				"8-9-etv0.1.2-stv1.2.3-getZkProof.json.success",
				"9-10-etv0.1.2-stv1.2.3-getZkProof.json.large.success",
			},
		},
		{
			Path: confM.Execution.DirTo(),
			Entries: []string{
				"0-1-getZkProof.json",
				"1-2-getZkProof.json",
				"8-9-getZkProof.json",
				"9-10-getZkProof.json",
			},
		},
		{
			Path:    confM.BlobDecompression.DirFrom(),
			Entries: []string{},
		},
		{
			Path: confM.BlobDecompression.DirDone(),
			Entries: []string{
				"0-2-bcv0.1.2-ccv0.1.2-getZkBlobCompressionProof.json.success",
				"2-4-bcv0.1.2-ccv0.1.2-getZkBlobCompressionProof.json.failure.code_2",
				"4-6-bcv0.1.2-ccv0.1.2-getZkBlobCompressionProof.json.failure.code_77",
				"6-8-bcv0.1.2-ccv0.1.2-getZkBlobCompressionProof.json.failure.code_137",
			},
		},
		{
			Path: confM.BlobDecompression.DirTo(),
			Entries: []string{
				"0-2-getZkBlobCompressionProof.json",
			},
		},
		{
			Path:    confM.Aggregation.DirFrom(),
			Entries: []string{},
		},
		{
			Path: confM.Aggregation.DirDone(),
			Entries: []string{
				"0-2-deadbeef57-getZkAggregatedProof.json.success",
				"2-4-deadbeef57-getZkAggregatedProof.json.failure.code_2",
				"4-6-deadbeef57-getZkAggregatedProof.json.failure.code_77",
				"6-8-deadbeef57-getZkAggregatedProof.json.failure.code_137",
			},
		},
		{
			Path:    confM.Aggregation.DirTo(),
			Entries: []string{"0-2-deadbeef57-getZkAggregatedProof.json"},
		},
	}

	for _, dirVal := range expectedStructure {
		dir, err := os.Open(dirVal.Path)
		require.NoErrorf(t, err, "dir %v", dirVal.Path)
		filesFound, err := dir.Readdirnames(-1)
		require.NoErrorf(t, err, "dir %v", dirVal.Path)
		slices.Sort(filesFound)
		assert.Equalf(t, dirVal.Entries, filesFound, "dir %v", dirVal.Path)
	}

}

func TestFileWatcherM(t *testing.T) {

	confM, _ := setupFsTest(t)

	// Create a list of files
	eFrom := confM.Execution.DirFrom
	cFrom := confM.BlobDecompression.DirFrom
	aFrom := confM.Aggregation.DirFrom

	exitCode := 0 // we are not interesting in the exit code here

	// The jobs, declared in the order in which they are expected to be found

	// Name of the expected inprogress files
	expectedFNames := []struct {
		FName string
		Skip  bool
	}{
		{
			FName: createTestInputFile(eFrom(), 0, 1, execJob, exitCode),
		},
		{
			Skip:  true, // wrong directory
			FName: createTestInputFile(eFrom(), 0, 1, aggregationJob, exitCode),
		},
		{
			FName: createTestInputFile(cFrom(), 0, 1, compressionJob, exitCode),
		},
		{
			FName: createTestInputFile(eFrom(), 1, 2, execJob, exitCode),
		},
		{
			FName: createTestInputFile(cFrom(), 1, 2, compressionJob, exitCode),
		},
		{
			FName: createTestInputFile(aFrom(), 0, 2, aggregationJob, exitCode),
		},
		{
			Skip:  true, // for large only
			FName: createTestInputFile(eFrom(), 2, 4, execJob, exitCode, forLarge),
		},
		{
			FName: createTestInputFile(eFrom(), 4, 5, execJob, exitCode),
		},
		{
			FName: createTestInputFile(cFrom(), 2, 5, compressionJob, exitCode),
		},
		{
			FName: createTestInputFile(aFrom(), 2, 5, aggregationJob, exitCode),
		},
	}

	fw := NewFsWatcher(confM)

	for _, f := range expectedFNames {
		if f.Skip {
			continue
		}
		found := fw.GetBest()
		if assert.NotNil(t, found, "did not find the job") {
			assert.Equal(t, f.FName, found.OriginalFile)
		}
	}
	assert.Nil(t, fw.GetBest(), "the queue should be empty now")
}

func TestFileWatcherL(t *testing.T) {

	_, confL := setupFsTest(t)

	// Create a list of files
	eFrom := confL.Execution.DirFrom()

	exitCode := 0 // we are not interesting in the exit code here

	// The jobs, declared in the order in which they are expected to be found

	// Name of the expected inprogress files
	expectedFNames := []struct {
		FName string
		Skip  bool
	}{
		{
			Skip:  true, // not large
			FName: createTestInputFile(eFrom, 0, 1, execJob, exitCode),
		},
		{
			Skip:  true, // wrong directory
			FName: createTestInputFile(eFrom, 0, 1, aggregationJob, exitCode),
		},
		{
			FName: createTestInputFile(eFrom, 1, 2, execJob, exitCode, forLarge),
		},
		{
			FName: createTestInputFile(eFrom, 2, 4, execJob, exitCode, forLarge),
		},
		{
			Skip:  true, // not large
			FName: createTestInputFile(eFrom, 4, 5, execJob, exitCode),
		},
		{
			Skip:  true, // wrong dir
			FName: createTestInputFile(eFrom, 2, 5, compressionJob, exitCode),
		},
	}

	fw := NewFsWatcher(confL)

	for _, f := range expectedFNames {
		if f.Skip {
			continue
		}
		found := fw.GetBest()
		if assert.NotNil(t, found, "did not find the job") {
			assert.Equal(t, f.FName, found.OriginalFile)
		}
	}
	assert.Nil(t, fw.GetBest(), "the queue should be empty now")
}

func setupFsTest(t *testing.T) (confM, confL *config.Config) {

	// Testdir is going to contain the whole test directory
	testDir := t.TempDir()

	const (
		dirlogs     = "logs"
		proverM     = "prover-full-M"
		proverL     = "prover-full-L"
		execution   = "execution"
		compression = "compression"
		aggregation = "aggregation"
	)

	// Create a configuration using temporary directories
	cmd := `
/bin/sh {{.InFile}}
CODE=$?
if [ $CODE -eq 0 ]; then
	touch {{.OutFile}}
fi
exit $CODE
`
	cmdLarge := `
	/bin/sh {{.InFile}}
	CODE=$?
	CODE=$(($CODE - 12))
	if [ $CODE -eq 0 ]; then
		touch {{.OutFile}}
	fi
	exit $CODE
	`

	cmdLargeInternal := `
/bin/sh {{.InFile}}
CODE=$?
CODE=$(($CODE - 10))
if [ $CODE -eq 0 ]; then
	touch {{.OutFile}}
fi
exit $CODE
`

	// For a prover M
	confM = &config.Config{
		Version: "0.2.4",

		Controller: config.Controller{
			EnableExecution:            true,
			EnableBlobDecompression:    true,
			EnableAggregation:          true,
			LocalID:                    proverM,
			Prometheus:                 config.Prometheus{Enabled: false},
			RetryDelays:                []int{0, 1},
			WorkerCmd:                  cmd,
			WorkerCmdLarge:             cmdLargeInternal,
			DeferToOtherLargeCodes:     []int{12, 137},
			RetryLocallyWithLargeCodes: []int{10, 77},
		},

		Execution: config.Execution{
			WithRequestDir: config.WithRequestDir{
				RequestsRootDir: path.Join(testDir, proverM, execution),
			},
		},
		BlobDecompression: config.BlobDecompression{
			WithRequestDir: config.WithRequestDir{
				RequestsRootDir: path.Join(testDir, proverM, compression),
			},
		},
		Aggregation: config.Aggregation{
			WithRequestDir: config.WithRequestDir{
				RequestsRootDir: path.Join(testDir, proverM, aggregation),
			},
		},
	}

	_confL := *confM
	confL = &_confL
	confL.Controller.LocalID = proverL
	confL.Controller.WorkerCmdLarge = cmdLarge
	confL.Execution.CanRunFullLarge = true

	// confL = &config.GlobalConfig{
	// 	Version: "0.2.4",

	// 	Controller: config.Controller{
	// 		EnableExecution:            true,
	// 		EnableBlobDecompression:    false,
	// 		EnableAggregation:          false,
	// 		LocalID:                    proverL,
	// 		Prometheus:                 config.Prometheus{Enabled: false},
	// 		RetryDelays:                []int{0, 1},
	// 		WorkerCmd:                  cmdLarge,
	// 		WorkerCmdLarge:             cmdLarge,
	// 		DeferToOtherLargeCodes:     []int{12, 137},
	// 		RetryLocallyWithLargeCodes: []int{10, 77},
	// 	},
	// 	Execution: config.Execution{
	// 		WithRequestDir: config.WithRequestDir{
	// 			RequestsRootDir: path.Join(testDir, proverM, execution),
	// 		},
	// 		CanRunFullLarge: true,
	// 	},
	// 	BlobDecompression: config.BlobDecompression{
	// 		WithRequestDir: config.WithRequestDir{
	// 			RequestsRootDir: path.Join(testDir, proverM, compression),
	// 		},
	// 	},
	// 	Aggregation: config.Aggregation{
	// 		WithRequestDir: config.WithRequestDir{
	// 			RequestsRootDir: path.Join(testDir, proverM, aggregation),
	// 		},
	// 	},
	// }

	// ensure the template are parsed
	confM.Controller.WorkerCmdTmpl = template.Must(template.New("worker").Parse(confM.Controller.WorkerCmd))
	confM.Controller.WorkerCmdLargeTmpl = template.Must(template.New("worker-large").Parse(confM.Controller.WorkerCmdLarge))
	confL.Controller.WorkerCmdTmpl = template.Must(template.New("worker").Parse(confL.Controller.WorkerCmd))
	confL.Controller.WorkerCmdLargeTmpl = template.Must(template.New("worker-large").Parse(confL.Controller.WorkerCmdLarge))

	// Initialize the dirs (and give them all permissions). They will be
	// wiped out after the test anyway.
	permCode := fs.FileMode(0777)
	err := errors.Join(
		os.MkdirAll(confM.Execution.DirFrom(), permCode),
		os.MkdirAll(confM.Execution.DirTo(), permCode),
		os.MkdirAll(confM.Execution.DirDone(), permCode),
		os.MkdirAll(confM.BlobDecompression.DirFrom(), permCode),
		os.MkdirAll(confM.BlobDecompression.DirTo(), permCode),
		os.MkdirAll(confM.BlobDecompression.DirDone(), permCode),
		os.MkdirAll(confM.Aggregation.DirFrom(), permCode),
		os.MkdirAll(confM.Aggregation.DirTo(), permCode),
		os.MkdirAll(confM.Aggregation.DirDone(), permCode),
	)

	if err != nil {
		t.Fatalf("could not create the temporary directories")
	}

	return confM, confL
}

const (
	execJob int = iota
	compressionJob
	aggregationJob
	forLarge bool = true
)

func createTestInputFile(
	dirfrom string,
	start, end, jobType, exitWith int,
	large ...bool,
) (fname string) {

	// The filenames are expected to match the regexp pattern that we have in
	// the job definition.
	fmtString := ""
	switch jobType {
	case execJob:
		fmtString = "%v-%v-etv0.1.2-stv1.2.3-getZkProof.json"
	case compressionJob:
		fmtString = "%v-%v-bcv0.1.2-ccv0.1.2-getZkBlobCompressionProof.json"
	case aggregationJob:
		fmtString = "%v-%v-deadbeef57-getZkAggregatedProof.json"
	default:
		panic("incorrect job type")
	}

	filename := fmt.Sprintf(fmtString, start, end)
	if len(large) > 0 && large[0] {
		filename += ".large"
	}
	f, err := os.Create(path.Join(dirfrom, filename))
	if err != nil {
		panic(err)
	}

	// If called (with the test configuration (i.e. with sh), the file will
	// immediately exit with the provided error code)
	f.WriteString(fmt.Sprintf("#!/bin/sh\nexit %v", exitWith))
	f.Close()

	return filename
}

func TestSpotInstanceMode(t *testing.T) {

	t.Skipf("this breaks the CI pipeline")

	var (
		cfg    = setupFsTestSpotInstance(t)
		nbTest = 5
	)

	for i := 0; i < nbTest; i++ {
		// Create the input file
		createTestInputFile(cfg.Execution.DirFrom(), i, i, execJob, 0)
	}

	// This bit of code finds the current process with the goal of self-SIGTERMING
	// a few seconds after we start the controller.
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		panic("could not find the current process")
	}

	go runController(context.Background(), cfg)

	// Wait for the controller to process 2 jobs and be in the middle of the
	// 3rd job.
	time.Sleep(5 * time.Second)

	if err := p.Signal(os.Signal(syscall.SIGTERM)); err != nil {
		panic("panic could not self-send a SIGTERM")
	}

	var (
		contentFrom, eFrom = lsname(cfg.Execution.DirFrom())
		contentDone, eDone = lsname(cfg.Execution.DirDone())
	)

	if eFrom != nil || eDone != nil {
		t.Fatalf("could not list the directories: %v %v", eFrom, eDone)
	}

	assert.Len(t, contentFrom, 3)
	assert.Len(t, contentDone, 2)
}

func setupFsTestSpotInstance(t *testing.T) (cfg *config.Config) {

	// Testdir is going to contain the whole test directory
	testDir := t.TempDir()

	cfg = &config.Config{
		Version: "0.2.4",

		Controller: config.Controller{
			SpotInstanceMode: true,
			EnableExecution:  true,
			LocalID:          "test-prover-id",
			Prometheus:       config.Prometheus{Enabled: false},
			RetryDelays:      []int{0, 1},
			WorkerCmdTmpl:    template.Must(template.New("test-cmd").Parse("sleep 2")),
		},

		Execution: config.Execution{
			WithRequestDir: config.WithRequestDir{
				RequestsRootDir: path.Join(testDir, "execution"),
			},
		},
	}

	// Initialize the dirs (and give them all permissions). They will be
	// wiped out after the test anyway.
	permCode := fs.FileMode(0777)
	err := errors.Join(
		os.MkdirAll(cfg.Execution.DirFrom(), permCode),
		os.MkdirAll(cfg.Execution.DirTo(), permCode),
		os.MkdirAll(cfg.Execution.DirDone(), permCode),
	)

	if err != nil {
		t.Fatal(err)
	}

	return cfg
}
