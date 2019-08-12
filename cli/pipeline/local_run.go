package pipeline

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/deliveroo/paddle/cli/data"
	"github.com/spf13/cobra"
	git "gopkg.in/src-d/go-git.v4"
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
	localRunCmd.Flags().StringVarP(&localRunCmdFlags.ImageTag, "tag", "t", "", "Image tag (overrides the one defined in the pipeline)")
	localRunCmd.Flags().StringVarP(&localRunCmdFlags.StepBranch, "step-branch", "B", "", "Step branch (overrides the one defined in the pipeline)")
	localRunCmd.Flags().StringVarP(&localRunCmdFlags.StepVersion, "step-version", "V", "", "Step version (overrides the one defined in the pipeline)")
	localRunCmd.Flags().BoolVarP(&localRunCmdFlags.TailLogs, "logs", "l", true, "Tail logs")
	localRunCmd.Flags().StringSliceVarP(&localRunCmdFlags.Secrets, "secret", "S", []string{}, "Secret to pull into the environment (in the form ENV_VAR:secret_store:key_name)")
	localRunCmdFlags.DeletePollInterval = defaultDeletePollInterval
	localRunCmdFlags.StartTimeout = defaultStartTimeout
}

func localRunPipeline(path string, flags *localRunCmdFlagsStruct) {
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
			step.OverrideBranch(flags.StepBranch, pipeline.OverrideInputs)
		}
		if flags.StepVersion != "" {
			step.OverrideVersion(flags.StepVersion, pipeline.OverrideInputs)
		}
		err = localRunPipelineStep(pipeline, &step, flags)
		if err != nil {
			logFatalf("[paddle] %s", err.Error())
		}
	}
}

func findBaseGitFolder() string {
	relPath := "/"
	found := false
	basePath, _ := os.Getwd()
	for !found {
		files, err := ioutil.ReadDir(basePath + relPath)
		if err != nil {
			log.Fatal(err)
		}
		for _, f := range files {
			if f.Name() == ".git" {
				found = true
				break
			}
		}
		if !found {
			relPath = relPath + "../"
		}
	}
	return relPath
}

func runAsyncCmd(outputPrefix string, cmdName string, cmdArgs ...string) {
	cmd := exec.Command(cmdName, cmdArgs...) //"docker-compose", "build", dockerImageName)
	fmt.Println(cmd)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatalf("could not get stderr pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("could not get stdout pipe: %v", err)
	}
	go func() {
		merged := io.MultiReader(stdout, stderr)
		scanner := bufio.NewScanner(merged)
		for scanner.Scan() {
			msg := scanner.Text()
			fmt.Printf("%s | %s\n", outputPrefix, msg)
		}
	}()
	if err := cmd.Run(); err != nil {
		log.Fatalf("could not run cmd: %v", err)
	}
}

func findGitBranch() string {
	relPath := findBaseGitFolder()
	r, err := git.PlainOpen("." + relPath)
	if err != nil {
		log.Fatal(err)
	}
	ref, _ := r.Head()
	branchName := ref.Name().Short()

	return branchName
}

func localRunPipelineStep(pipeline *PipelineDefinition, step *PipelineDefinitionStep, flags *localRunCmdFlagsStruct) error {
	log.Printf("[paddle] Running step %s", step.Step)

	// Do checksum to not download unless needed
	for _, input := range step.Inputs {
		data.CopyPathToDestinationWithoutS3Path(
			pipeline.Bucket, input.Step, input.Version, input.Branch, input.Path,
			"inputs", []string{}, "",
		)
	}

	fmt.Println(pipeline.CurrentBranch)
	dockerImageName := strings.Split(strings.Split(step.Image, "/")[1], ":")[0]
	fmt.Println(dockerImageName)
	fmt.Println(step.Image)

	//res, err := exec.Command("aws", "ecr", "get-login", "--profile", "k8s_production", "--no-include-email", "--region", "eu-west-1").Output()

	//s := strings.Split(strings.TrimSuffix(string(res), "\n"), " ")
	//runAsyncCmd("aws login", s[0], s[1:]...)
	//runAsyncCmd("docker build", "docker-compose", "build", dockerImageName)

	//runAsyncCmd("docker build", "docker", "tag", dockerImageName+":latest", step.Image)
	//runAsyncCmd("docker build", "docker", "push", step.Image)

	fmt.Println("done")

	//podDefinition := NewPodDefinition(pipeline, step)
	for _, cmd := range step.Commands {
		arr := []string{
			"run", "-T", "-e", "INPUT_PATH=/app/inputs", "-e", "OUTPUT_PATH=/app/outputs", dockerImageName}
		arr = append(arr, strings.Split(cmd, " ")...)
		runAsyncCmd(step.Step, "docker-compose", arr...)
	}

	return nil
}
