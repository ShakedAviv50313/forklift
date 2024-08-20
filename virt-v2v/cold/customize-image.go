package main

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed scripts
var scriptFS embed.FS

const (
	WIN_FIRSTBOOT_PATH = "/Program Files/Guestfs/Firstboot/"
)

// CustomizeWindows customizes a windows disk image by uploading scripts and configuring new RunOnce task to execute it.
//
// The function writes two bash scripts to the specified local tmp directory,
// uploads them to the disk image using `virt-customize`.
//
// NOTE: We can't use the firstboot commands as the system is not ready to run all commands.
// Some commands for example `Get-Disk` returns empty list. So we need to set the registry with `RunOnce` task,
// which will be run by the Winodws once the system is ready.
//
// Arguments:
//   - domain (string): The VM which should be customized.
//
// Returns:
//   - error: An error if something goes wrong during the process, or nil if successful.
func CustomizeWindows(domain string) error {
	fmt.Printf("Customizing domain '%s'", domain)
	t := EmbedTool{filesystem: &scriptFS}
	err := t.CreateFilesFromFS(DIR)
	if err != nil {
		return err
	}
	windowsScriptsPath := filepath.Join(DIR, "scripts", "windows")
	initPath := filepath.Join(windowsScriptsPath, "restore_config_init.bat")
	restoreScriptPath := filepath.Join(windowsScriptsPath, "restore_config.ps1")

	// Upload scripts to the windows
	uploadScriptPath := fmt.Sprintf("%s:%s", restoreScriptPath, WIN_FIRSTBOOT_PATH)
	uploadInitPath := fmt.Sprintf("%s:%s", initPath, WIN_FIRSTBOOT_PATH)

	var extraArgs []string
	extraArgs = append(extraArgs, getScriptArgs("upload", uploadScriptPath, uploadInitPath)...)
	err = CustomizeDomainExec(domain, extraArgs...)
	if err != nil {
		return err
	}
	// Run the virt-win-reg to update the Windows registry with our new RunOnce tas
	taskPath := filepath.Join(windowsScriptsPath, "task.reg")
	err = VirtWinRegExec(domain, taskPath)
	if err != nil {
		return err
	}
	return nil
}

// getScriptArgs generates a list of arguments.
//
// Arguments:
//   - argName (string): Argument name which should be used for all the values
//   - values (...string): The list of values which should be joined with argument names.
//
// Returns:
//   - []string: List of arguments
//
// Example:
//   - getScriptArgs("firstboot", boot1, boot2) => ["--firstboot", boot1, "--firstboot", boot2]
func getScriptArgs(argName string, values ...string) []string {
	var args []string
	for _, val := range values {
		args = append(args, fmt.Sprintf("--%s", argName), val)
	}
	return args
}

// VirtWinRegExec executes `virt-win-reg` to edit the windows registries.
//
// Arguments:
//   - domain (string): The VM domain in which should be the registries changed.
//   - taskPath (...string): The path to the task registry which should be merged with the Windows registry.
//
// Returns:
//   - error: An error if something goes wrong during the process, or nil if successful.
func VirtWinRegExec(domain string, taskPath string) error {
	customizeCmd := exec.Command("virt-win-reg", "--merge", domain, taskPath)
	customizeCmd.Stdout = os.Stdout
	customizeCmd.Stderr = os.Stderr

	fmt.Println("exec:", customizeCmd)
	if err := customizeCmd.Run(); err != nil {
		return fmt.Errorf("error executing virt-win-reg command: %w", err)
	}
	return nil
}

// CustomizeDomainExec executes `virt-customize` to customize the image.
//
// Arguments:
//   - domain (string): The VM domain which should be customized.
//   - extraArgs (...string): The additional arguments which will be appended to the `virt-customize` arguments.
//
// Returns:
//   - error: An error if something goes wrong during the process, or nil if successful.
func CustomizeDomainExec(domain string, extraArgs ...string) error {
	args := []string{"--verbose", "--domain", domain}
	args = append(args, extraArgs...)
	customizeCmd := exec.Command("virt-customize", args...)
	customizeCmd.Stdout = os.Stdout
	customizeCmd.Stderr = os.Stderr

	fmt.Println("exec:", customizeCmd)
	if err := customizeCmd.Run(); err != nil {
		return fmt.Errorf("error executing virt-customize command: %w", err)
	}
	return nil
}
