package main

import (
	"io/ioutil"
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	// disable logs in test
	log.SetOutput(ioutil.Discard)

	// set some env variables for using in tests
	os.Setenv("BUILDKITE_COMMIT", "123")
	os.Setenv("BUILDKITE_MESSAGE", "fix: temp file not correctly deleted")
	os.Setenv("BUILDKITE_BRANCH", "go-rewrite")
	os.Setenv("env3", "env-3")
	os.Setenv("env4", "env-4")
	os.Setenv("TEST_MODE", "true")

	run := m.Run()

	os.Exit(run)
}

func mockGeneratePipeline(steps []Step, plugin Plugin) (*os.File, error) {
	mockFile, _ := os.Create("pipeline.txt")
	defer mockFile.Close()

	return mockFile, nil
}

func TestUploadPipelineCallsBuildkiteAgentCommand(t *testing.T) {
	plugin := Plugin{Diff: "echo ./foo-service"}
	cmd, args, err := uploadPipeline(plugin, mockGeneratePipeline)

	assert.Equal(t, "buildkite-agent", cmd)
	assert.Equal(t, []string{"pipeline", "upload", "pipeline.txt"}, args)
	assert.Equal(t, err, nil)
}

func TestUploadPipelineCallsBuildkiteAgentCommandWithInterpolation(t *testing.T) {
	plugin := Plugin{Diff: "echo ./foo-service", Interpolation: true}
	cmd, args, err := uploadPipeline(plugin, mockGeneratePipeline)

	assert.Equal(t, "buildkite-agent", cmd)
	assert.Equal(t, []string{"pipeline", "upload", "pipeline.txt", "--no-interpolation"}, args)
	assert.Equal(t, err, nil)
}

func TestUploadPipelineCancelsIfThereIsNoDiffOutput(t *testing.T) {
	plugin := Plugin{Diff: "echo"}
	cmd, args, err := uploadPipeline(plugin, mockGeneratePipeline)

	assert.Equal(t, "", cmd)
	assert.Equal(t, []string{}, args)
	assert.Equal(t, err, nil)
}

func TestDiff(t *testing.T) {
	want := []string{
		"services/foo/serverless.yml",
		"services/bar/config.yml",
		"ops/bar/config.yml",
		"README.md",
	}

	got, err := diff(`echo services/foo/serverless.yml
services/bar/config.yml

ops/bar/config.yml
README.md`)

	assert.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestPipelinesToTriggerGetsListOfPipelines(t *testing.T) {
	want := []string{"service-1", "service-2", "service-4"}

	watch := []WatchConfig{
		{
			Paths: []string{"watch-path-1"},
			Step:  Step{Trigger: "service-1"},
		},
		{
			Paths: []string{"watch-path-2/", "watch-path-3/", "watch-path-4"},
			Step:  Step{Trigger: "service-2"},
		},
		{
			Paths: []string{"watch-path-5"},
			Step:  Step{Trigger: "service-3"},
		},
		{
			Paths: []string{"watch-path-2"},
			Step:  Step{Trigger: "service-4"},
		},
	}

	changedFiles := []string{
		"watch-path-1/text.txt",
		"watch-path-2/.gitignore",
		"watch-path-2/src/index.go",
		"watch-path-4/test/index_test.go",
	}

	pipelines := stepsToTrigger(changedFiles, watch)
	var got []string

	for _, v := range pipelines {
		got = append(got, v.Trigger)
	}

	assert.Equal(t, want, got)
}

func TestGeneratePipeline(t *testing.T) {
	steps := []Step{
		{
			Trigger: "foo-service-pipeline",
			Build:   Build{Message: "build message"},
		},
	}

	want :=
		`steps:
- trigger: foo-service-pipeline
  build:
    message: build message
- wait
- command: echo "hello world"
- command: cat ./file.txt`

	plugin := Plugin{
		Wait: true,
		Hooks: []HookConfig{
			{Command: "echo \"hello world\""},
			{Command: "cat ./file.txt"},
		},
	}

	pipeline, err := generatePipeline(steps, plugin)
	defer os.Remove(pipeline.Name())

	if err != nil {
		assert.Equal(t, true, false, err.Error())
	}

	got, _ := ioutil.ReadFile(pipeline.Name())

	assert.Equal(t, want, string(got))
}
