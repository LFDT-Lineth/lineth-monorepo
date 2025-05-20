package controller

import (
	"context"
	"testing"
	"text/template"
	"time"

	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/stretchr/testify/assert"
)

func TestRetryWithLarge(t *testing.T) {

	// A test command useful for testing the command generation
	var testDefinition = JobDefinition{
		// Give a name to the command
		Name: jobNameExecution,
		// The template of the output file (returns a constant template with no
		// parameters)
		OutputFileTmpl: template.Must(
			template.New("output-file").
				Parse("output-fill-constant"),
		),
		RequestsRootDir: "./testdata",
	}

	jobs := []struct {
		Job
		ExpCode int
	}{
		{
			Job: Job{
				Def:        &testDefinition,
				LockedFile: "exit-0.sh",
				// Not directly needed but helpful to track the process name
				Start: 0,
				End:   0,
			},
			ExpCode: 0,
		},
		{
			Job: Job{
				Def:        &testDefinition,
				LockedFile: "exit-1.sh",
				// Not directly needed but helpful to track the process name
				Start: 1,
				End:   1,
			},
			ExpCode: 1,
		},
		{
			Job: Job{
				Def:        &testDefinition,
				LockedFile: "exit-77.sh",
				// Not directly needed but helpful to track the process name
				Start: 2,
				End:   2,
			},
			ExpCode: 77 + 10,
		},
		{
			Job: Job{
				Def:        &testDefinition,
				LockedFile: "sigkill.sh",
				// Not directly needed but helpful to track the process name
				Start: 3,
				End:   3,
			},
			ExpCode: 137,
		},
	}

	e := NewExecutor(&config.Config{
		Controller: config.Controller{
			WorkerCmdTmpl: template.Must(
				template.New("test-cmd").
					Parse("/bin/sh {{.InFile}}"),
			),
			// And the large fields. The commands adds a +10 to the return code
			// to leave an evidence that the return code was obtained through
			// running the large command.
			WorkerCmdLargeTmpl: template.Must(
				template.New("test-cmd-large").
					Parse(`/bin/sh -c "/bin/sh {{.InFile}}"; exit $(($? + 10))`),
			),
			RetryLocallyWithLargeCodes: config.DefaultRetryLocallyWithLargeCodes,
		},

		// Execution: config.Execution{
		// 	CanRunFullLarge: true,
		// },
	})

	for i := range jobs {
		status := e.Run(context.Background(), &jobs[i].Job)
		assert.Equalf(t, jobs[i].ExpCode, status.ExitCode, "got status %++v", status)
	}
}

func TestEarlyExitOnSpotInstanceMode(t *testing.T) {

	// A test command useful for testing the command generation
	testDefinition := JobDefinition{
		// Give a name to the command
		Name: jobNameExecution,
		// The template of the output file (returns a constant template with no
		// parameters)
		OutputFileTmpl: template.Must(
			template.New("output-file").
				Parse("output-fill-constant"),
		),
		RequestsRootDir: "./testdata",
	}

	e := NewExecutor(&config.Config{
		Controller: config.Controller{
			WorkerCmdTmpl: template.Must(
				template.New("test-cmd").
					Parse("/bin/sh {{.InFile}}"),
			),
		},
	})

	job := &Job{
		Def:        &testDefinition,
		LockedFile: "sleep-4.sh",
		// Not directly needed but helpful to track the process name
		Start: 0,
		End:   0,
	}

	var (
		// The context auto-cancels after 2 seconds and will not let the original
		// command finish.
		ctx, cancelMainExpiration = context.WithTimeout(context.Background(), 2*time.Second)
	)

	status := e.Run(ctx, job)
	cancelMainExpiration()

	assert.Equal(t, CodeKilledByUs, status.ExitCode)

	// We wait 3 more second to ensure all sub-process have exited
	time.Sleep(3 * time.Second)
}
