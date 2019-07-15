package pipeline

import (
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/deliveroo/paddle/cli/data"
	"github.com/spf13/cobra"
)

type localRunCmdFlagsStruct struct {
	StepName           string
	BucketName         string
	ImageTag           string
	StepBranch         string
	StepVersion        string
	OverrideInputs     bool
	TailLogs           bool
	Secrets            []string
	Env                []string
	BucketOverrides    []string
	DeletePollInterval time.Duration
	StartTimeout       time.Duration
}

var localRunCmd = &cobra.Command{
	Use:   "local-run [pipeline_yaml]",
	Short: "Run locally pipeline step",
	Args:  cobra.ExactArgs(1),
	Long: `Run a pipeline step locally.

Example:

$ paddle pipeline local-run test_pipeline.yaml
`,
	Run: func(cmd *cobra.Command, args []string) {
		localRunPipeline(args[0], localRunCmdFlags)
	},
}

var localRunCmdFlags *localRunCmdFlagsStruct

func init() {
	localRunCmdFlags = &localRunCmdFlagsStruct{}
	localRunCmd.Flags().StringVarP(&localRunCmdFlags.StepName, "step", "s", "", "Single step to execute")
	localRunCmd.Flags().StringVarP(&localRunCmdFlags.BucketName, "bucket", "b", "", "Bucket name")
	localRunCmd.Flags().StringVarP(&localRunCmdFlags.ImageTag, "tag", "t", "", "Image tag (overrides the one defined in the pipeline)")
	localRunCmd.Flags().StringVarP(&localRunCmdFlags.StepBranch, "step-branch", "B", "", "Step branch (overrides the one defined in the pipeline)")
	localRunCmd.Flags().StringVarP(&localRunCmdFlags.StepVersion, "step-version", "V", "", "Step version (overrides the one defined in the pipeline)")
	localRunCmd.Flags().BoolVarP(&localRunCmdFlags.TailLogs, "logs", "l", true, "Tail logs")
	localRunCmd.Flags().BoolVarP(&localRunCmdFlags.OverrideInputs, "override-inputs", "I", false, "Override input version/branch (only makes sense to use with -B or -V)")
	localRunCmd.Flags().StringSliceVarP(&localRunCmdFlags.Secrets, "secret", "S", []string{}, "Secret to pull into the environment (in the form ENV_VAR:secret_store:key_name)")
	localRunCmd.Flags().StringSliceVarP(&localRunCmdFlags.Env, "env", "e", []string{}, "Environment variables to set (in the form name:value)")
	localRunCmd.Flags().StringSliceVar(&localRunCmdFlags.BucketOverrides, "replace-input-buckets", []string{}, "Override input bucket names (in the form original_bucket_name:new_bucket_name)")
	localRunCmdFlags.DeletePollInterval = defaultDeletePollInterval
	localRunCmdFlags.StartTimeout = defaultStartTimeout
}

func localRunPipeline(path string, flags *localRunCmdFlagsStruct) {
	fmt.Println("waaaaaat")
	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err.Error())
	}
	pipeline := ParsePipeline(data)
	if flags.BucketName != "" {
		pipeline.Bucket = flags.BucketName
	}

	for _, step := range pipeline.Steps {
		if flags.StepName != "" && step.Step != flags.StepName {
			continue
		}
		if flags.ImageTag != "" {
			step.OverrideTag(flags.ImageTag)
		}
		if flags.StepBranch != "" {
			step.OverrideBranch(flags.StepBranch, flags.OverrideInputs)
		}
		if flags.StepVersion != "" {
			step.OverrideVersion(flags.StepVersion, flags.OverrideInputs)
		}
		err = localRunPipelineStep(pipeline, &step, flags)
		if err != nil {
			logFatalf("[paddle] %s", err.Error())
		}
	}
}

func localRunPipelineStep(pipeline *PipelineDefinition, step *PipelineDefinitionStep, flags *localRunCmdFlagsStruct) error {
	log.Printf("[paddle] Running step %s", step.Step)

	//podDefinition := NewPodDefinition(pipeline, step)
	for _, input := range step.Inputs {
		data.CopyPathToDestinationWithoutS3Path(
			input.Bucket, input.Step, input.Version, input.Branch, input.Path,
			"inputs", []string{}, "",
		)
		fmt.Printf("%+v", input)
	}
	return nil
}
