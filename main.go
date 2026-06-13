package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"golang.org/x/sys/windows/registry"
)

// Installation mirrors the JSON objects in the "installations" array.
type Installation struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	QuickName string `json:"qickname"`
	Path      string `json:"path"`
}

// Config represents the top-level structure of config.json.
type Config struct {
	Active        int            `json:"active"`
	Installations []Installation `json:"installations"`
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("could not get user home directory: %v", err)
	}

	configDir := filepath.Join(homeDir, ".jvm")
	configPath := filepath.Join(configDir, "config.json")

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		log.Fatalf("could not create config directory %s: %v", configDir, err)
	}

	cmd := strings.ToLower(os.Args[1])

	switch cmd {
	case "help", "--help", "-h":
		printHelp()
	case "ls", "list":
		cfg := mustReadConfig(configPath)
		full := false
		if len(os.Args) > 2 && os.Args[2] == "full" {
			full = true
		}
		listInstallations(cfg, full)
	case "active", "a", "current", "c":
		cfg := mustReadConfig(configPath)
		full := false
		if len(os.Args) > 2 && os.Args[2] == "full" {
			full = true
		}
		showActive(cfg, full)
	case "switch", "s":
		if len(os.Args) < 3 {
			fmt.Println("Error: missing installation identifier")
			fmt.Println("Usage: jvm switch <id|name|quickname>")
			os.Exit(1)
		}
		cfg := mustReadConfig(configPath)
		if err := switchActive(cfg, os.Args[2], configPath, homeDir); err != nil {
			log.Fatalf("switch failed: %v", err)
		}
		fmt.Println("Switched successfully.")
	case "setup":
		if err := setupPath(homeDir); err != nil {
			log.Fatalf("setup failed: %v", err)
		}
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printHelp()
		os.Exit(1)
	}
}

func mustReadConfig(path string) *Config {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("config file not found at %s. Run 'jvm setup' first? (or create the file manually)", path)
		}
		log.Fatalf("error reading config: %v", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("error parsing config: %v", err)
	}
	return &cfg
}

func printHelp() {
	fmt.Println("jvm - Java Version Manager (Windows, non-admin)")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  jvm <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  help                  Show this help message")
	fmt.Println("  ls|list [quick|full]  List all installations (default quick)")
	fmt.Println("  active|a|current|c [quick|full]  Show active installation (default quick)")
	fmt.Println("  switch|s <id|name|quickname>   Switch active installation")
	fmt.Printf("  setup                 Ensure %%USERPROFILE%%\\Java is in your user PATH\n")
}

// listInstallations prints a table of installations.
func listInstallations(cfg *Config, full bool) {
	if len(cfg.Installations) == 0 {
		fmt.Println("No installations found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if full {
		fmt.Fprintln(w, "ID\tName\tPath")
	} else {
		fmt.Fprintln(w, "ID\tQuickName\tPath")
	}
	for _, inst := range cfg.Installations {
		if full {
			fmt.Fprintf(w, "%d\t%s\t%s\n", inst.ID, inst.Name, inst.Path)
		} else {
			fmt.Fprintf(w, "%d\t%s\t%s\n", inst.ID, inst.QuickName, inst.Path)
		}
	}
	w.Flush()
}

// showActive displays the currently active installation.
func showActive(cfg *Config, full bool) {
	idx := findInstallationByID(cfg.Installations, cfg.Active)
	if idx == -1 {
		fmt.Printf("Active ID %d not found in installations.\n", cfg.Active)
		return
	}
	inst := cfg.Installations[idx]
	if full {
		fmt.Printf("ID: %d\nName: %s\nPath: %s\n", inst.ID, inst.Name, inst.Path)
	} else {
		fmt.Printf("ID: %d\nQuickName: %s\nPath: %s\n", inst.ID, inst.QuickName, inst.Path)
	}
}

// switchActive updates the active installation and the symlink.
func switchActive(cfg *Config, arg string, configPath string, homeDir string) error {
	// Find the installation matching the argument.
	idx := findInstallation(cfg.Installations, arg)
	if idx == -1 {
		return fmt.Errorf("no installation found matching '%s'", arg)
	}
	inst := cfg.Installations[idx]

	// Check that the target path exists and is a directory.
	info, err := os.Stat(inst.Path)
	if err != nil {
		return fmt.Errorf("target path does not exist: %s", inst.Path)
	}
	if !info.IsDir() {
		return fmt.Errorf("target path is not a directory: %s", inst.Path)
	}

	// Update the config.
	cfg.Active = inst.ID
	if err := writeConfig(configPath, cfg); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	// Update the symlink.
	javaLink := filepath.Join(homeDir, "Java")
	if err := updateSymlink(javaLink, inst.Path); err != nil {
		return fmt.Errorf("failed to update symlink: %v", err)
	}

	// Ensure JAVA_HOME is set correctly.
	if err := setJAVA_HOME(homeDir); err != nil {
		return err
	}

	return nil
}

// findInstallation locates an installation by ID (if numeric) or by case-insensitive
// exact match against QuickName or Name. Returns the index or -1 if not found.
// If multiple matches exist, it returns the first one.
func findInstallation(installations []Installation, arg string) int {
	// Try as ID first.
	if id, err := strconv.Atoi(arg); err == nil {
		return findInstallationByID(installations, id)
	}

	argLower := strings.ToLower(arg)
	for i, inst := range installations {
		if strings.ToLower(inst.QuickName) == argLower || strings.ToLower(inst.Name) == argLower {
			return i
		}
	}
	return -1
}

func findInstallationByID(installations []Installation, id int) int {
	for i, inst := range installations {
		if inst.ID == id {
			return i
		}
	}
	return -1
}

func writeConfig(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// updateSymlink ensures that linkPath is a symlink pointing to target.
// If linkPath exists as a symlink/junction, it is replaced.
// If it exists but is not a reparse point (e.g., regular directory), an error is returned.
func updateSymlink(linkPath, target string) error {
	// Check if linkPath already exists.
	fi, err := os.Lstat(linkPath)
	if err == nil {
		// It exists. Check if it's a symlink/junction (reparse point).
		if fi.Mode()&fs.ModeSymlink == 0 && fi.Mode()&os.ModeDir == 0 {
			// Possibly a file, not our concern. We'll remove it.
			// But safest is to only replace directories/symlinks.
			if fi.Mode().IsRegular() {
				return fmt.Errorf("%s exists and is a regular file; cannot replace with symlink", linkPath)
			}
			// It's a directory but not a symlink. Don't delete.
			return fmt.Errorf("%s is a real directory. Refusing to replace it. Please remove it manually if you want to use a symlink.", linkPath)
		}
		// It's a symlink or junction. Remove it.
		if err := os.Remove(linkPath); err != nil {
			return fmt.Errorf("failed to remove existing symlink %s: %v", linkPath, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat %s: %v", linkPath, err)
	}

	// Create the directory symlink.
	if err := os.Symlink(target, linkPath); err != nil {
		return fmt.Errorf("failed to create symlink %s -> %s: %v (developer mode must be enabled)", linkPath, target, err)
	}
	return nil
}

// setupPath ensures that %USERPROFILE%\Java\bin is in the user's PATH.
func setupPath(homeDir string) error {
	javaBinDir := filepath.Join(homeDir, "Java", "bin")

	// Open the user's Environment key (HKCU\Environment).
	k, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("cannot open registry: %v", err)
	}
	defer k.Close()

	// Read current PATH value (may be absent).
	currentPath, _, err := k.GetStringValue("Path")
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("error reading PATH: %v", err)
	}

	// Check if javaBinDir is already present (semicolon-separated, case-insensitive).
	paths := strings.Split(currentPath, ";")
	for _, p := range paths {
		if strings.EqualFold(strings.TrimSpace(p), javaBinDir) {
			fmt.Printf("%s is already in your PATH.\n", javaBinDir)

			// Ensure JAVA_HOME is set correctly.
			if err := setJAVA_HOME(homeDir); err != nil {
				return err
			}

			return nil
		}
	}

	// Append javaBinDir.
	var newPath string
	if currentPath == "" {
		newPath = javaBinDir
	} else {
		newPath = currentPath + ";" + javaBinDir
	}

	if err := k.SetStringValue("Path", newPath); err != nil {
		return fmt.Errorf("failed to update PATH: %v", err)
	}

	fmt.Printf("Added %s to your user PATH.\n", javaBinDir)

	// Ensure JAVA_HOME is set correctly.
	if err := setJAVA_HOME(homeDir); err != nil {
		return err
	}

	return nil
}

// setJAVA_HOME ensures the user environment variable JAVA_HOME points to the symlink %USERPROFILE%\Java.
// If JAVA_HOME is already set to a different value, the user is asked before overwriting.
func setJAVA_HOME(homeDir string) error {
	javaHomeDir := filepath.Join(homeDir, "Java")

	k, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("cannot open registry: %v", err)
	}
	defer k.Close()

	existingValue, _, err := k.GetStringValue("JAVA_HOME")
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("error reading JAVA_HOME: %v", err)
	}

	// Already set correctly? Nothing to do.
	if strings.EqualFold(existingValue, javaHomeDir) {
		fmt.Printf("JAVA_HOME is already set to %s\n", javaHomeDir)
		return nil
	}

	// If it's set to something else, ask.
	if existingValue != "" {
		fmt.Printf("JAVA_HOME is currently set to %q.\n", existingValue)
		fmt.Printf("Do you want to change it to %q? [y/N]: ", javaHomeDir)

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		input = strings.ToLower(input)

		if input != "y" && input != "yes" {
			fmt.Println("JAVA_HOME left unchanged.")
			return nil
		}
	}

	// Set the new value.
	if err := k.SetStringValue("JAVA_HOME", javaHomeDir); err != nil {
		return fmt.Errorf("failed to set JAVA_HOME: %v", err)
	}

	fmt.Printf("JAVA_HOME set to %s\n", javaHomeDir)
	broadcastEnvChange()
	return nil
}

// broadcastEnvChange notifies the user that they may need to log out or restart for environment variable changes to take effect.
func broadcastEnvChange() {
	fmt.Println("(You may need to log out/restart for environment variables to be fully refreshed.)")
}
