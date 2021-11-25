package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"github.com/google/uuid"
)

var (
	numEnvironments int
	factor          int
	features        int
	segments        int
	targets         int
)

func init() {
	flag.IntVar(&numEnvironments, "environments", 2, "the number of environments to generate")
	flag.IntVar(&factor, "factor", 2, "the factor to apply to the number of features, segments and targets for each environmnet that's created")
	flag.IntVar(&features, "features", 1, "baseline number of features to generate")
	flag.IntVar(&segments, "segments", 1, "baseline number of segments to generate")
	flag.IntVar(&targets, "targets", 1, "baseline number of targets to generate")
	flag.Parse()
}

func main() {
	environment := uuid.New().String()
	project := uuid.New().String()
	makeEnv(environment, project, features, segments, targets)

	// Here we generate the desired number of environments with the config that
	// they need and we apply the factor value to the number of segments/targets/features
	// that get created for each env. See the README for more detail.
	for i := 1; i < numEnvironments; i++ {
		features = features * factor
		segments = segments * factor
		targets = targets * factor
		environment := uuid.New().String()
		project := uuid.New().String()

		makeEnv(environment, project, features, segments, targets)
	}
}

func makeEnv(environment string, project string, numFeatures int, numSegments int, numTargets int) {
	featureBuf := &bytes.Buffer{}
	segmentBuf := &bytes.Buffer{}
	targetBuf := &bytes.Buffer{}

	createFeatureConfigs(features, environment, project, featureBuf)
	createSegments(segments, environment, segmentBuf)
	createTargets(targets, environment, project, targetBuf)

	dirName := filepath.Clean(fmt.Sprintf("env-%s", environment))
	if err := os.Mkdir(dirName, 0750); err != nil {
		log.Fatalf("failed to create env dir: %s", err)
	}

	featureFilename := fmt.Sprintf("%s/feature_config.json", dirName)
	copyToFile(featureFilename, featureBuf)

	segmentFilename := fmt.Sprintf("%s/segments.json", dirName)
	copyToFile(segmentFilename, segmentBuf)

	targetFilename := fmt.Sprintf("%s/targets.json", dirName)
	copyToFile(targetFilename, targetBuf)

	readmeBuf := &bytes.Buffer{}

	readme := readme{
		Environment: environment,
		NumFeatures: features,
		NumSegments: segments,
		NumTargets:  targets,
	}
	parseTemplate(readmeTemplate, readme, readmeBuf)

	readmeFilename := fmt.Sprintf("%s/README.md", dirName)
	copyToFile(readmeFilename, readmeBuf)

}

var readmeTemplate = `
# Env-{{ .Environment }}

- Number of Features - {{ .NumFeatures }}
- Number of Segments - {{ .NumSegments }}
- Number of Targets - {{ .NumTargets }}
`

type readme struct {
	Environment string
	NumFeatures int
	NumSegments int
	NumTargets  int
}

func parseTemplate(tmpl string, v interface{}, w io.Writer) {
	funcMap := template.FuncMap{
		"subtract": subtract,
	}

	t, err := template.New("temp").Funcs(funcMap).Parse(tmpl)
	if err != nil {
		log.Fatal(err)
	}

	if err := t.Execute(w, v); err != nil {
		log.Fatal(err)
	}
}

// copyToFile copies the contents of the reader to a file
func copyToFile(filename string, w io.Reader) {
	f, err := os.OpenFile(filepath.Clean(filename), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		f.Close()
		log.Fatalf("failed to open segments.json: %s", err)
	}

	io.Copy(f, w)
	f.Close()
}

var targetTemplate = `
[
{{- $size := subtract (len .) 1 -}}
{{- range $index, $target := . -}}

{
	"account": "foo",
	"attributes": {
	  "age": 56,
	  "ages": [
		1,
		2,
		3
	  ],
	  "happy": true,
	  "host": "foo.com",
	  "userGroups": [
		"Foo",
		"Volvo",
		"BMW"
	  ]
	},
	"anonymous": false,
	"createdAt": 1634222520273,
	"environment": "{{ $target.Environment }}",
	"identifier": "{{ $target.Identifier }}",
	"name": "{{ $target.Name }}",
	"org": "bar",
	"project": "{{ $target.Project }}",
	"segments": []
	}{{if (lt $index $size)}},{{end}}
{{end}}
]
`

type target struct {
	Project     string
	Environment string
	Identifier  string
	Name        string
}

func createTargets(n int, environment string, project string, w io.Writer) {
	targets := []target{}
	for i := 0; i < n; i++ {
		t := target{
			Project:     project,
			Environment: environment,
			Identifier:  fmt.Sprintf("target-%d", i),
			Name:        fmt.Sprintf("name-%d", i),
		}
		targets = append(targets, t)
	}

	funcMap := template.FuncMap{
		"subtract": subtract,
	}

	tmpl, err := template.New("targets").Funcs(funcMap).Parse(targetTemplate)
	if err != nil {
		log.Fatal(err)
	}

	if err := tmpl.Execute(w, targets); err != nil {
		log.Fatal(err)
	}
}

var segmentTemplate = `
[
{{- $size := subtract (len .) 1 -}}
{{- range $index, $segment := . -}}
{
	"environment": "{{ $segment.Environment }}",
	"excluded": [],
	"identifier": "{{ $segment.Identifier }}",
	"included": [],
	"name": "{{ $segment.Name }}",
	"rules": [
{
	"attribute": "ip",
		"id": "31c18ee7-8051-44cc-8507-b44580467ee5",
		"negate": false,
		"op": "equal",
		"values": [
		  "2a00:23c5:b672:2401:158:f2a6:67a0:6a79"
		]
}
],
	"version": {{ $index }},
	"createdAt": 123,
	"modifiedAt": 456
}{{if (lt $index $size)}},{{end}}
{{ end }}
]
`

type segment struct {
	Environment string
	Identifier  string
	Name        string
}

var featureConfigBoolTemplate = `
[
{{- $size := subtract (len .) 1 -}}
{{- range $index, $feature := . -}}
{
  "project": "{{ $feature.Project }}",
  "environment": "{{ $feature.Environment }}",
  "feature": "{{ $feature.Identifier }}",
  "state": "{{ $feature.State }}",
  "kind": "boolean",
  "variations": [
    {
      "identifier": "true",
      "value": "true",
      "name": "True"
    },
	{
      "identifier": "false",
      "value": "false",
      "name": "False"
    }
  ],
  "rules": [],
  "defaultServe": {
    "variation": "true"
  },
  "offVariation": "false",
  "prerequisites": [],
  "variationToTargetMap": [],
  "version": {{ $index }}
}{{if (lt $index $size)}},{{end}}
{{end}}
]
`

func createSegments(n int, environment string, w io.Writer) {
	segments := []segment{}
	for i := 0; i < n; i++ {
		segment := segment{
			Environment: environment,
			Identifier:  fmt.Sprintf("segment-%d", i),
			Name:        fmt.Sprintf("name-%d", i),
		}

		segments = append(segments, segment)
	}

	funcMap := template.FuncMap{
		"subtract": subtract,
	}

	tmpl, err := template.New("segments").Funcs(funcMap).Parse(segmentTemplate)
	if err != nil {
		log.Fatal(err)
	}

	if err := tmpl.Execute(w, segments); err != nil {
		log.Fatal(err)
	}
}

type feature struct {
	Project     string
	Environment string
	Identifier  string
	State       string
}

func createFeatureConfigs(n int, environment string, project string, w io.Writer) {
	features := []feature{}
	for i := 0; i < n; i++ {
		f := feature{
			Project:     project,
			Environment: environment,
			Identifier:  fmt.Sprintf("feature-%d", i),
			State:       "on",
		}
		features = append(features, f)
	}

	funcMap := template.FuncMap{
		"subtract": subtract,
	}

	tmpl, err := template.New("features").Funcs(funcMap).Parse(featureConfigBoolTemplate)
	if err != nil {
		log.Fatal(err)
	}

	if err := tmpl.Execute(w, features); err != nil {
		log.Fatal(err)
	}
}

func subtract(a int, b int) int {
	return a - b
}
