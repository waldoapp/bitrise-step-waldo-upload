package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/bitrise-io/go-utils/command"
)

const (
	stepName    = "Waldo Upload Bitrise Step"
	stepVersion = "2.4.1"

	stepAssetBaseURL = "https://github.com/waldoapp/waldo-go-agent/releases"

	stepMaxDownloadAttempts = 2
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

type UploadMetadata struct {
	AppID        string    `json:"appID"`
	AppVersionID string    `json:"appVersionID"`
	Host         string    `json:"host"`
	UploadTime   time.Time `json:"uploadTime"`
}

func checkInputs() error {
	if len(inputBuildPath) == 0 {
		return fmt.Errorf("Missing required input: ‘build_path’")
	}

	if len(inputUploadToken) == 0 {
		inputUploadToken = os.Getenv("WALDO_UPLOAD_TOKEN")
	}

	if len(inputUploadToken) == 0 {
		return fmt.Errorf("Missing required input: ‘upload_token’")
	}

	return nil
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

func determineUploadArgs() []string {
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

	return args
}

func determineWorkingPath() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("WaldoUpdate-%d", os.Getpid()))
}

func displayVersion() {
	if stepVerbose {
		fmt.Printf("%s %s (%s/%s)\n\n", stepName, stepVersion, stepPlatform, stepArch)
	}
}

func downloadAgent(retryAllowed bool) (bool, error) {
	// fmt.Printf("Downloading latest Waldo Agent…\n\n")

	client := &http.Client{}

	req, err := http.NewRequest("GET", stepAssetURL, nil)

	var resp *http.Response

	if err == nil {
		dumpRequest(req, false)

		resp, err = client.Do(req)
	}

	if retryAllowed && err != nil {
		emitError(err)

		return true, nil // did not succeed but retry is allowed
	}

	if err == nil {
		defer resp.Body.Close()

		dumpResponse(resp, false)

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			err = fmt.Errorf("Unable to download Waldo Agent, HTTP status: %s", resp.Status)

			if retryAllowed && shouldRetry(resp) {
				emitError(err)

				return true, nil // did not succeed but retry is allowed
			}

			return false, err
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
		return false, fmt.Errorf("Unable to download Waldo Agent, error: %v, url: %s", err, stepAssetURL)
	}

	return false, nil // don’t bother to retry
}

func downloadAgentWithRetry() error {
	var err error

	for attempts := 1; attempts <= stepMaxDownloadAttempts; attempts++ {
		retry, err := downloadAgent(attempts < stepMaxDownloadAttempts)

		if !retry || err != nil {
			break
		}

		fmt.Printf("\nFailed download attempts: %d -- retrying…\n\n", attempts)
	}

	return err
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

func emitError(err error) {
	fmt.Printf("\n") // flush stdout

	os.Stderr.WriteString(fmt.Sprintf("waldo-upload: %v\n", err))
}

func enrichEnvironment() []string {
	env := os.Environ()

	setEnvironVar(&env, "WALDO_WRAPPER_NAME_OVERRIDE", stepName)
	setEnvironVar(&env, "WALDO_WRAPPER_VERSION_OVERRIDE", stepVersion)

	return env
}

func execAgent() error {
	args := determineUploadArgs()

	cmd := exec.Command(stepAgentPath, args...)

	cmd.Env = enrichEnvironment()
	cmd.Stderr = os.Stderr

	stdout, err := prepareStdout(cmd)

	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	data, err := readLastNonEmptyLine(stdout)

	if err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	buildID, err := extractBuildID(data)

	if err != nil {
		return err
	}

	return exportEnvironmentWithEnvman("WALDO_BUILD_ID", buildID)
}

func exportEnvironmentWithEnvman(keyStr, valueStr string) error {
	cmd := command.New("envman", "add", "--key", keyStr)

	cmd.SetStdin(strings.NewReader(valueStr))

	return cmd.Run()
}

func extractBuildID(data []byte) (string, error) {
	um := &UploadMetadata{}

	if err := json.Unmarshal(data, um); err != nil {
		return "", err
	}

	return um.AppVersionID, nil
}
func fail(err error) {
	emitError(err)

	if ee, ok := err.(*exec.ExitError); ok {
		os.Exit(ee.ExitCode())
	}

	os.Exit(1)
}

func main() {
	defer func() {
		if err := recover(); err != nil {
			fail(fmt.Errorf("Unhandled panic: %v", err))
		}
	}()

	displayVersion()

	if err := performUpload(); err != nil {
		fail(err)
	}
}

func performUpload() error {
	if err := checkInputs(); err != nil {
		return err
	}

	if err := prepareSource(); err != nil {
		return err
	}

	if err := prepareTarget(); err != nil {
		return err
	}

	defer cleanupTarget()

	if err := downloadAgentWithRetry(); err != nil {
		return err
	}

	return execAgent()

}

func prepareSource() error {
	stepAssetURL = determineAssetURL()

	return nil
}

func prepareStdout(cmd *exec.Cmd) (io.Reader, error) {
	pr, err := cmd.StdoutPipe()

	if err != nil {
		return nil, err
	}

	return io.TeeReader(pr, os.Stdout), nil
}

func prepareTarget() error {
	stepWorkingPath = determineWorkingPath()
	stepAgentPath = determineAgentPath()

	err := os.RemoveAll(stepWorkingPath)

	if err != nil {
		return err
	}

	return os.MkdirAll(stepWorkingPath, 0755)
}

func readLastNonEmptyLine(r io.Reader) ([]byte, error) {
	scanner := bufio.NewScanner(r)

	var lastLine []byte

	for scanner.Scan() {
		if line := scanner.Bytes(); len(line) > 0 {
			lastLine = scanner.Bytes()
		}
	}
	if err := scanner.Err(); err != nil {
		return []byte{}, err
	}

	return lastLine, nil
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

func shouldRetry(resp *http.Response) bool {
	switch resp.StatusCode {
	case 408, 429, 500, 502, 503, 504:
		return true

	default:
		return false
	}
}
