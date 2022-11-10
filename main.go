package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	stepName    = "Waldo Upload Bitrise Step"
	stepVersion = "2.3.0"

	stepAssetBaseURL = "https://github.com/waldoapp/waldo-go-agent/releases"
)

var (
	stepAgentPath   string
	stepAssetURL    string
	stepWorkingPath string

	stepArch         = detectArch()
	stepAssetVersion = detectAssetVersion()
	stepPlatform     = detectPlatform()
	stepVerbose      = detectVerbose()

	inputBuildPath   = os.Getenv("build_path")
	inputGitCommit   = os.Getenv("git_commit")
	inputGitBranch   = os.Getenv("git_branch")
	inputUploadToken = os.Getenv("upload_token")
	inputVariantName = os.Getenv("variant_name")
	inputVerbose     = os.Getenv("is_debug_mode") == "yes"
)

func checkInputs() {
	if len(inputBuildPath) == 0 {
		fail(fmt.Errorf("Missing required input: ‘build_path’"))
	}

	if len(inputUploadToken) == 0 {
		inputUploadToken = os.Getenv("WALDO_UPLOAD_TOKEN")
	}

	if len(inputUploadToken) == 0 {
		fail(fmt.Errorf("Missing required input: ‘upload_token’"))
	}
}

func cleanupTarget() {
	os.RemoveAll(stepWorkingPath)
}

func detectArch() string {
	arch := runtime.GOARCH

	switch arch {
	case "amd64":
		return "x86_64"

	default:
		return arch
	}
}

func detectAssetVersion() string {
	if version := os.Getenv("WALDO_UPLOAD_ASSET_VERSION"); len(version) > 0 {
		return version
	}

	return "latest"
}

func detectPlatform() string {
	platform := runtime.GOOS

	switch platform {
	case "darwin":
		return "macOS"

	default:
		return strings.Title(platform)
	}
}

func detectVerbose() bool {
	if verbose := os.Getenv("WALDO_UPLOAD_VERBOSE"); verbose == "1" {
		return true
	}

	return false
}

func determineAgentPath() string {
	agentName := "waldo-agent"

	if stepPlatform == "windows" {
		agentName += ".exe"
	}

	return filepath.Join(stepWorkingPath, "waldo-agent")
}

func determineAssetURL() string {
	assetName := fmt.Sprintf("waldo-agent-%s-%s", stepPlatform, stepArch)

	if stepPlatform == "windows" {
		assetName += ".exe"
	}

	assetBaseURL := stepAssetBaseURL

	if stepAssetVersion != "latest" {
		assetBaseURL += "/download/" + stepAssetVersion
	} else {
		assetBaseURL += "/latest/download"
	}

	return assetBaseURL + "/" + assetName
}

func determineWorkingPath() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("WaldoUpdate-%d", os.Getpid()))
}

func displayVersion() {
	if stepVerbose {
		fmt.Printf("%s %s (%s/%s)\n", stepName, stepVersion, stepPlatform, stepArch)
	}
}

func downloadAgent() {
	// fmt.Printf("\nDownloading latest Waldo Agent…\n\n")

	client := &http.Client{}

	req, err := http.NewRequest("GET", stepAssetURL, nil)

	var resp *http.Response

	if err == nil {
		dumpRequest(req, false)

		resp, err = client.Do(req)
	}

	if err == nil {
		defer resp.Body.Close()

		dumpResponse(resp, false)

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			fail(fmt.Errorf("Unable to download Waldo Agent, HTTP status: %s", resp.Status))
		}
	}

	var file *os.File = nil

	if err == nil {
		file, err = os.OpenFile(stepAgentPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0775)
	}

	if err == nil {
		defer file.Close()

		_, err = io.Copy(file, resp.Body)
	}

	if err != nil {
		fail(fmt.Errorf("Unable to download Waldo Agent, error: %v, url: %s", err, stepAssetURL))
	}
}

func dumpRequest(req *http.Request, body bool) {
	if stepVerbose {
		dump, err := httputil.DumpRequestOut(req, body)

		if err == nil {
			fmt.Printf("\n--- Request ---\n%s\n", dump)
		}
	}
}

func dumpResponse(resp *http.Response, body bool) {
	if stepVerbose {
		dump, err := httputil.DumpResponse(resp, body)

		if err == nil {
			fmt.Printf("\n--- Response ---\n%s\n", dump)
		}
	}
}

func enrichEnvironment() []string {
	env := os.Environ()

	setEnvironVar(&env, "WALDO_WRAPPER_NAME_OVERRIDE", stepName)
	setEnvironVar(&env, "WALDO_WRAPPER_VERSION_OVERRIDE", stepVersion)

	return env
}

func execAgent() {
	args := []string{"upload"}

	if len(inputGitBranch) > 0 {
		args = append(args, "--git_branch", inputGitBranch)
	}

	if len(inputGitCommit) > 0 {
		args = append(args, "--git_commit", inputGitCommit)
	}

	if len(inputUploadToken) > 0 {
		args = append(args, "--upload_token", inputUploadToken)
	}

	if len(inputVariantName) > 0 {
		args = append(args, "--variant_name", inputVariantName)
	}

	if inputVerbose {
		args = append(args, "--verbose")
	}

	if buildPath := os.Getenv("build_path"); len(buildPath) > 0 {
		args = append(args, buildPath)
	}

	cmd := exec.Command(stepAgentPath, args...)

	cmd.Env = enrichEnvironment()
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	err := cmd.Run()

	if ee, ok := err.(*exec.ExitError); ok {
		os.Exit(ee.ExitCode())
	}
}

func fail(err error) {
	fmt.Printf("\n") // flush stdout

	os.Stderr.WriteString(fmt.Sprintf("waldo-upload: %v\n", err))

	os.Exit(1)
}

func main() {
	defer func() {
		if err := recover(); err != nil {
			fail(fmt.Errorf("Unhandled panic: %v", err))
		}
	}()

	displayVersion()

	checkInputs()

	prepareSource()
	prepareTarget()

	defer cleanupTarget()

	downloadAgent()
	execAgent()
}

func prepareSource() {
	stepAssetURL = determineAssetURL()
}

func prepareTarget() {
	stepWorkingPath = determineWorkingPath()
	stepAgentPath = determineAgentPath()

	err := os.RemoveAll(stepWorkingPath)

	if err == nil {
		err = os.MkdirAll(stepWorkingPath, 0755)
	}

	if err != nil {
		fail(err)
	}
}

func setEnvironVar(env *[]string, key, value string) {
	for idx := range *env {
		if strings.HasPrefix((*env)[idx], key+"=") {
			(*env)[idx] = key + "=" + value

			return
		}
	}

	*env = append(*env, key+"="+value)
}
