package cmd

import (
	log "github.com/sirupsen/logrus"

	"github.com/ghodss/yaml"
	"github.com/gruntwork-io/terragrunt/cli"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/spf13/cobra"

	"golang.org/x/sync/errgroup"

	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Parse env vars into a map
func getEnvs() map[string]string {
	envs := os.Environ()
	m := make(map[string]string)

	for _, env := range envs {
		results := strings.Split(env, "=")
		m[results[0]] = results[1]
	}

	return m
}

// Represents an entire config file
type AtlantisConfig struct {
	// Version of the config syntax
	Version int `json:"version"`

	// If Atlantis should merge after finishing `atlantis apply`
	AutoMerge bool `json:"automerge"`

	// The project settings
	Projects []AtlantisProject `json:"projects,omitempty"`
}

// Represents an Atlantis Project directory
type AtlantisProject struct {
	// The directory with the terragrunt.hcl file
	Dir string `json:"dir"`

	// Define workflow name
	Workflow string `json:"workflow,omitempty"`

	// Autoplan settings for which plans affect other plans
	Autoplan AutoplanConfig `json:"autoplan"`
}

// Autoplan settings for which plans affect other plans
type AutoplanConfig struct {
	// Relative paths from this modules directory to modules it depends on
	WhenModified []string `json:"when_modified"`

	// If autoplan should be enabled for this dir
	Enabled bool `json:"enabled"`
}

// Terragrunt imports can be relative or absolute
// This makes relative paths absolute
func makePathAbsolute(path string, parentPath string) string {
	if strings.HasPrefix(path, gitRoot) {
		return path
	}

	parentDir := filepath.Dir(parentPath)
	return filepath.Join(parentDir, path)
}

// Parses the terragrunt config at <path> to find all modules it depends on
func getDependencies(path string) ([]string, error) {
	decodeTypes := []config.PartialDecodeSectionType{
		config.DependencyBlock,
		config.DependenciesBlock,
		config.TerraformBlock,
	}

	options, err := options.NewTerragruntOptions(path)
	if err != nil {
		return nil, err
	}
	options.RunTerragrunt = cli.RunTerragrunt
	options.Env = getEnvs()

	parsedConfig, err := config.PartialParseConfigFile(path, options, nil, decodeTypes)
	if err != nil {
		return nil, err
	}

	// if theres no terraform source and we're ignoring parent terragrunt configs
	// return nils to indicate we should skip this project
	if (parsedConfig.Terraform == nil || parsedConfig.Terraform.Source == nil) && ignoreParentTerragrunt == true {
		return nil, nil
	}

	dependencies := []string{}

	if parsedConfig.Dependencies != nil {
		dependencies = parsedConfig.Dependencies.Paths
	}

	if parsedConfig.Terraform != nil && parsedConfig.Terraform.Source != nil {
		source := parsedConfig.Terraform.Source
		// TODO: Make more robust. Check for bitbucket, etc.
		if !strings.Contains(*source, "git") {
			dependencies = append(dependencies, *source)
		}
	}

	return dependencies, nil
}

// Creates an AtlantisProject for a directory
func createProject(sourcePath string) (*AtlantisProject, error) {
	dependencies, err := getDependencies(sourcePath)
	if err != nil {
		return nil, err
	}
	// if dependecies AND err is nil then return nils to indicate we should skip this project
	if err == nil && dependencies == nil {
		return nil, nil
	}

	absoluteSourceDir := filepath.Dir(sourcePath)

	// All dependencies depend on their own .hcl file, and any tf files in their directory
	relativeDependencies := []string{
		"*.hcl",
		"*.tf*",
	}

	// Add other dependencies based on their relative paths
	for _, dependencyPath := range dependencies {
		absolutePath := makePathAbsolute(dependencyPath, sourcePath)
		relativePath, err := filepath.Rel(absoluteSourceDir, absolutePath)
		if err != nil {
			return nil, err
		}
		relativeDependencies = append(relativeDependencies, relativePath)
	}

	relativeSourceDir := strings.TrimPrefix(absoluteSourceDir, gitRoot)

	project := &AtlantisProject{
		Dir:      relativeSourceDir,
		Workflow: workflow,
		Autoplan: AutoplanConfig{
			Enabled:      autoPlan,
			WhenModified: relativeDependencies,
		},
	}
	return project, nil
}

// Finds the absolute paths of all terragrunt.hcl files
func getAllTerragruntFiles() ([]string, error) {
	options, err := options.NewTerragruntOptions(gitRoot)
	if err != nil {
		return nil, err
	}

	paths, err := config.FindConfigFilesInPath(gitRoot, options)
	if err != nil {
		return nil, err
	}

	return paths, nil
}

func main(cmd *cobra.Command, args []string) error {
	// Ensure the gitRoot has a trailing slash and is an absolute path
	absoluteGitRoot, err := filepath.Abs(gitRoot)
	if err != nil {
		return err
	}
	gitRoot = absoluteGitRoot + "/"

	terragruntFiles, err := getAllTerragruntFiles()
	if err != nil {
		return err
	}

	config := AtlantisConfig{
		Version:   3,
		AutoMerge: false,
	}

	lock := sync.Mutex{}
	errGroup, _ := errgroup.WithContext(context.Background())

	// Concurrently looking all dependencies
	for _, terragruntPath := range terragruntFiles {
		terragruntPath := terragruntPath // https://golang.org/doc/faq#closures_and_goroutines

		errGroup.Go(func() error {
			project, err := createProject(terragruntPath)
			if err != nil {
				return err
			}
			// if project and err are nil then skip this project
			if err == nil && project == nil {
				return nil
			}

			// Lock the list as only one goroutine should be writing to config.Projects at a time
			lock.Lock()
			defer lock.Unlock()

			log.Info("Created project for ", terragruntPath)
			config.Projects = append(config.Projects, *project)

			return nil
		})

		if err := errGroup.Wait(); err != nil {
			return err
		}
	}

	// Convert config to YAML string
	yamlString, err := yaml.Marshal(&config)
	if err != nil {
		return err
	}

	// Write output
	if len(outputPath) != 0 {
		ioutil.WriteFile(outputPath, yamlString, 0644)
	} else {
		log.Println(string(yamlString))
	}

	return nil
}

var gitRoot string
var autoPlan bool
var ignoreParentTerragrunt bool
var workflow string
var outputPath string

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Makes atlantis config",
	Long:  `Logs Yaml representing Atlantis config to stderr`,
	RunE:  main,
}

func init() {
	rootCmd.AddCommand(generateCmd)

	pwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	generateCmd.PersistentFlags().BoolVar(&autoPlan, "autoplan", false, "Enable auto plan. Default is disabled")
	generateCmd.PersistentFlags().BoolVar(&ignoreParentTerragrunt, "ignore-parent-terragrunt", false, "Ignore parent terragrunt configs (those which don't reference a terraform module). Default is disabled")
	generateCmd.PersistentFlags().StringVar(&workflow, "workflow", "", "Name of the workflow to be customized in the atlantis server. Default is to not set")
	generateCmd.PersistentFlags().StringVar(&outputPath, "output", "", "Path of the file where configuration will be generated. Default is not to write to file")
	generateCmd.PersistentFlags().StringVar(&gitRoot, "root", pwd, "Path to the root directory of the github repo you want to build config for. Default is current dir")
}

// Runs a set of arguments, returning the output
func RunWithFlags(args []string) ([]byte, error) {
	randomInt := rand.Int()
	filename := fmt.Sprintf("test_artifacts/%d.yaml", randomInt)

	defer os.Remove(filename)

	allArgs := append([]string{
		"generate",
		"--output",
		filename,
	}, args...)
	rootCmd.SetArgs(allArgs)
	rootCmd.Execute()

	return ioutil.ReadFile(filename)
}
