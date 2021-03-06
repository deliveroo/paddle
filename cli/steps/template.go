package steps

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/deliveroo/paddle/cli/pipeline"
)

type PodSecret struct {
	Name  string
	Store string
	Key   string
}

type PodEnvVariable struct {
	Name  string
	Value string
}

type PodDefinition struct {
	PipelineName    string
	RunIdentifier   string
	PodName         string
	StepName        string
	StepVersion     string
	StepInputs      string
	BranchName      string
	Namespace       string
	Bucket          string
	SnsArn          string
	Secrets         []PodSecret
	Env             []PodEnvVariable
	BucketOverrides map[string]string

	Step pipeline.PipelineDefinitionStep
}

func (d *PodDefinition) needsVolume() bool {
	return d.Step.Resources.Storage != 0
}

const podTemplate = `
apiVersion: v1
kind: Pod
metadata:
  name: "{{ .PodName }}"
  namespace: {{ .Namespace }}
  labels:
    canoe.executor: paddle
    canoe.step.name: {{ .StepName }}
    canoe.step.branch: {{ .BranchName }}
    canoe.step.version: {{ .StepVersion }}
spec:
  restartPolicy: Never
  volumes:
    -
      name: shared-data
      {{ if ne .Step.Resources.Storage 0 }}
      persistentVolumeClaim:
        claimName: {{ .PodName }}-volume-claim
      {{ else }}
      emptyDir:
        medium: ''
      {{ end }}
    -
      name: docker-sock
      hostPath:
        path: /var/run/docker.sock
  containers:
    -
      name: main
      image: "{{ .Step.Image }}"
      imagePullPolicy: Always
      volumeMounts:
        -
          name: shared-data
          mountPath: /data
        -
          name: docker-sock
          mountPath: /var/run/docker.sock
      resources:
        limits:
          cpu: "{{ .Step.Resources.CPU }}"
          memory: "{{ .Step.Resources.Memory }}"
      command:
        - ((
          {{ range $index, $command := .Step.Commands }}
          ({{ $command }}) &&
          {{ end }}
          ))"
      env:
        -
          name: TASK_NAME
          value: {{ .StepName }}
        -
          name: STATE_MACHINE_ID
          value: {{ .PipelineName }}
        -
          name: BUCKET
          value: {{ .Bucket }}
        -
          name: EXECUTION_PATH
          value: {{ .BranchName }}/{{ .RunIdentifier }}/{{ .StepName }}
        -
          name: INPUT_PATH
          value: /data/input
        -
          name: OUTPUT_PATH
          value: /data/output
        -
          name: INPUTS
          value: {{ .StepInputs }}
        -
          name: OUTPUT
          value: {{ .StepName }}
        -
          name: SNS_TOPIC_ARN
          value: {{ .SnsArn }}
        -
          name: AWS_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: aws-credentials-training
              key: aws-access-key-id
        -
          name: AWS_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: aws-credentials-training
              key: aws-secret-access-key
        {{ range $index, $secret := .Secrets }}
        -
          name: {{ $secret.Name }}
          valueFrom:
            secretKeyRef:
              name: {{ $secret.Store }}
              key: {{ $secret.Key }}
        {{ end }}
        {{ range $index, $var := .Env }}
        -
          name: {{ $var.Name }}
          value: {{ $var.Value }}
        {{ end }}
`

const volumeTemplate = `
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: {{ .PodName }}-volume-claim
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: {{ .Step.Resources.Storage }}Mi
  persistentVolumeReclaimPolicy: Delete
`

func NewPodDefinition(pipelineDefinition *pipeline.PipelineDefinition, pipelineDefinitionStep *pipeline.PipelineDefinitionStep, snsArn string) *PodDefinition {
	stepName := sanitizeName(pipelineDefinitionStep.Step)
	branchName := sanitizeName(pipelineDefinitionStep.Branch)
	stepVersion := sanitizeName(pipelineDefinitionStep.Version)
	pipelineName := sanitizeName(pipelineDefinition.Pipeline)
	inputSteps := []string{}
	for _, input := range pipelineDefinitionStep.Inputs {
		inputSteps = append(inputSteps, input.Step)
	}
	stepInputs := strings.Join(inputSteps, ",")
	podName := fmt.Sprintf("%s-%s-%s-%s", sanitizeName(pipelineDefinition.Pipeline), sanitizeName(pipelineDefinitionStep.Version), stepName, branchName)

	return &PodDefinition{
		PipelineName:    pipelineName,
		PodName:         podName,
		Namespace:       pipelineDefinition.Namespace,
		Step:            *pipelineDefinitionStep,
		StepInputs:      stepInputs,
		Bucket:          pipelineDefinition.Bucket,
		SnsArn:          snsArn,
		StepName:        stepName,
		StepVersion:     stepVersion,
		BranchName:      branchName,
		Secrets:         []PodSecret{},
		BucketOverrides: map[string]string{},
	}
}

func (p PodDefinition) compile() *bytes.Buffer {
	fmap := template.FuncMap{
		"sanitizeName": sanitizeName,
		"bucketParam":  p.bucketParam,
	}
	tmpl := template.Must(template.New("podTemplate").Funcs(fmap).Parse(podTemplate))
	buffer := new(bytes.Buffer)
	err := tmpl.Execute(buffer, p)
	if err != nil {
		panic(err.Error())
	}
	return buffer
}

func (p PodDefinition) compileVolumeClaim() *bytes.Buffer {
	tmpl := template.Must(template.New("volumeTemplate").Parse(volumeTemplate))
	buffer := new(bytes.Buffer)
	err := tmpl.Execute(buffer, p)
	if err != nil {
		panic(err.Error())
	}
	return buffer
}

func (p *PodDefinition) parseSecrets(secrets []string) {
	for _, secret := range secrets {
		secretParts := strings.Split(secret, ":")

		p.Secrets = append(p.Secrets, PodSecret{
			Name:  secretParts[0],
			Store: secretParts[1],
			Key:   secretParts[2],
		})
	}
}

func (p *PodDefinition) setBucketOverrides(bucketOverrides []string) {
	for _, bucketOverride := range bucketOverrides {
		override := strings.Split(bucketOverride, ":")
		p.BucketOverrides[override[0]] = override[1]
	}
}

func (p *PodDefinition) parseEnv(env []string) {
	for _, v := range env {
		varParts := strings.Split(v, ":")

		p.Env = append(p.Env, PodEnvVariable{
			Name:  varParts[0],
			Value: varParts[1],
		})
	}
}

func (p *PodDefinition) bucketParam(bucket string) string {
	if bucket != "" {
		if bucketReplacement, exists := p.BucketOverrides[bucket]; exists {
			bucket = bucketReplacement
		}
		return "--bucket " + bucket
	}
	return ""
}

func sanitizeName(name string) string {
	str := strings.ToLower(name)
	str = strings.Replace(str, "_", "-", -1)
	str = strings.Replace(str, "/", "-", -1)
	return str
}
