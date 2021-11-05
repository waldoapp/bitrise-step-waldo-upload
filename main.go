package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/waldoapp/waldo-go-lib"
)

const (
	wrapperName     = "Go Step"
	wrapperNameFull = "Waldo Upload Bitrise Step"
	wrapperVersion  = "2.0.0"
)

var (
	waldoBuildPath        string
	waldoBuildPayloadPath string
	waldoBuildSuffix      string
	waldoFlavor           string
	waldoGitAccess        string
	waldoGitBranch        string
	waldoGitCommit        string
	waldoPlatform         string
	waldoUploadToken      string
	waldoVariantName      string
	waldoVerbose          bool
	waldoWorkingPath      string
)

func checkInputs() {
	waldoBuildPath = os.Getenv("build_path")
	waldoUploadToken = os.Getenv("upload_token")
	waldoVariantName = os.Getenv("variant_name")
	waldoGitCommit = os.Getenv("git_commit")
	waldoGitBranch = os.Getenv("git_branch")
	waldoVerbose = os.Getenv("is_debug_mode") == "true"

	if len(waldoBuildPath) == 0 {
		fail(fmt.Errorf("Missing required input: ‘build_path’"))
	}

	if len(waldoUploadToken) == 0 {
		waldoUploadToken = os.Getenv("WALDO_UPLOAD_TOKEN")
	}

	if len(waldoUploadToken) == 0 {
		fail(fmt.Errorf("Missing required input: ‘upload_token’"))
	}
}

func displaySummary(uploader *waldo.Uploader) {
	fmt.Printf("\n")
	fmt.Printf("Build path:          %s\n", summarize(uploader.BuildPath()))
	fmt.Printf("Git branch:          %s\n", summarize(uploader.GitBranch()))
	fmt.Printf("Git commit:          %s\n", summarize(uploader.GitCommit()))
	fmt.Printf("Upload token:        %s\n", summarizeSecure(uploader.UploadToken()))
	fmt.Printf("Variant name:        %s\n", summarize(uploader.VariantName()))

	if waldoVerbose {
		fmt.Printf("\n")
		fmt.Printf("Build payload path:  %s\n", summarize(uploader.BuildPayloadPath()))
		fmt.Printf("Inferred git branch: %s\n", summarize(uploader.InferredGitBranch()))
		fmt.Printf("Inferred git commit: %s\n", summarize(uploader.InferredGitCommit()))
	}

	fmt.Printf("\n")
}

func displayVersion() {
	fmt.Printf("%s %s / %s\n", wrapperNameFull, wrapperVersion, waldo.Version())
}

func fail(err error) {
	fmt.Printf("\n") // flush stdout

	os.Stderr.WriteString(fmt.Sprintf("waldo: %v\n", err))

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

	uploader := waldo.NewUploader(
		waldoBuildPath,
		waldoUploadToken,
		waldoVariantName,
		waldoGitCommit,
		waldoGitBranch,
		waldoVerbose,
		map[string]string{
			"wrapperName":    wrapperName,
			"wrapperVersion": wrapperVersion})

	if err := uploader.Validate(); err != nil {
		fail(err)
	}

	displaySummary(uploader)

	fmt.Printf("Uploading build to Waldo\n")

	if err := uploader.Upload(); err != nil {
		fail(err)
	}

	fmt.Printf("\nBuild ‘%s’ successfully uploaded to Waldo!\n", filepath.Base(waldoBuildPath))

	os.Exit(0)
}

func summarize(value string) string {
	if len(value) > 0 {
		return fmt.Sprintf("‘%s’", value)
	} else {
		return "(none)"
	}
}

func summarizeSecure(value string) string {
	if len(value) == 0 {
		return "(none)"
	}

	if !waldoVerbose {
		prefixLen := len(value)

		if prefixLen > 6 {
			prefixLen = 6
		}

		prefix := value[0:prefixLen]
		suffixLen := len(value) - len(prefix)
		secure := "********************************"

		value = prefix + secure[0:suffixLen]
	}

	return fmt.Sprintf("‘%s’", value)
}
