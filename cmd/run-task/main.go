package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"
	"runtime"
	"strings"
	"time"
)

type VcsOption struct {
	Repository string
	Name       string
	// Add other fields as required.
}

const (
	secretBaseURLTPL    = "http://taskcluster/secrets/v1/secret/%s"
	cacheUIDGIDMismatch = `
There is a UID/GID mismatch on the cache. This likely means:

a) different tasks are running as a different user/group
b) different Docker images have different UID/GID for the same user/group

Our cache policy is that the UID/GID for ALL tasks must be consistent
for the lifetime of the cache. This eliminates permissions problems due
to file/directory user/group ownership.

To make this error go away, ensure that all Docker images are use
a consistent UID/GID and that all tasks using this cache are running as
the same user/group.`
	nonEmptyVolume = `
error: volume %s is not empty

Our Docker image policy requires volumes to be empty.

The volume was likely populated as part of building the Docker image.
Change the Dockerfile and anything run from it to not create files in
any VOLUME.

A lesser possibility is that you stumbled upon a TaskCluster platform bug
where it fails to use new volumes for tasks.`
	fetchContentNotFound = `
error: fetch-content script not found

The script at 'taskcluster/scripts/misc/fetch-content' could not be
detected in the current environment.`
	exitPurgeCache = 72
	nullRevision   = "0000000000000000000000000000000000000000"
)

var (
	githubSSHFingerprint = []byte(`github.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCj7ndNxQowgcQnjshcLrqPEiiphnt+VTTvDP6mHBL9j1aNUkY4Ue1gvwnGLVlOhGeYrnZaMgRK6+PKCUXaDbC7qtbW8gIkhL7aGCsOr/C56SJMy/BCZfxd1nWzAOxSDPgVsmerOBYfNqltV9/hWCqBywINIR+5dIg6JTJ72pcEpEjcYgXkE2YEFXV1JHnsKgbLWNlhScqb2UmyRkQyytRLtL+38TGxkxCflmO+5Z8CSSNY7GidjMIZ7Q4zMjA2n1nGrlTDkzwDCsw+wqFPGQA179cnfGWOWRVruj16z6XyvxvjJwbz0wQZ75XK5tKSb7FNyeIEs4TT4jk+S4dhPeAUC5y+bDYirYgM4GC7uEnztnZyaVWQ7B381AK4Qdrwt51ZqExKbQpTUNn+EjqoTwvqNj4kqx5QUCI0ThS/YkOxJCXmPUWZbhjpCg56i+2aB6CmK2JGhn57K5mj0MNdBXA4/WnwH6XoPWJzK5Nyu2zB3nAZp+S5hpQs+p1vN1/wsjk=\n`)
	isMacOSX             = runtime.GOOS == "darwin"
	isPOSIX              = runtime.GOOS == "linux" || runtime.GOOS == "darwin"
	isWindows            = runtime.GOOS == "windows"
)

func printLine(prefix, m string) {
	now := time.Now().UTC().Format(time.RFC3339Nano)

	// Slicing the microseconds to 3 decimals.
	if dotIndex := strings.LastIndex(now, "."); dotIndex != -1 {
		now = now[:dotIndex+4] + "Z"
	}

	fmt.Printf("[%s %s] %s\n", prefix, now, m)
}

func addVcsArguments(vcsCheckouts, vcsSparseProfiles map[string]*string, project, name string) {
	checkoutFlagName := fmt.Sprintf("%s-checkout", project)
	sparseProfileFlagName := fmt.Sprintf("%s-sparse-profile", project)
	vcsCheckouts[project] = flag.String(checkoutFlagName, "", fmt.Sprintf("Directory where %s checkout should be created", name))
	vcsSparseProfiles[project] = flag.String(sparseProfileFlagName, "", fmt.Sprintf("Path to sparse profile for %s checkout", name))
}

// collectVcsOptions is a stub function that collects VCS options based on the provided arguments.
func collectVcsOptions(user, repository, name string) VcsOption {
	// TODO: Implement logic to actually collect VCS options.

	// For now, returning a default VcsOption.
	return VcsOption{
		Repository: repository,
		Name:       name,
	}
}

func main() {
	args := os.Args[1:] // os.Args[0] is the program name

	currentDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}
	os.Setenv("TASK_WORKDIR", currentDir)
	printLine("setup", "run-task started in " + currentDir)
	printLine("setup", "Invoked by command:" + strings.Join(args, " "))

	// 2. Check for UID and running as root.
	currentUser, _ := user.Current()
	uid := currentUser.Uid
	runningAsRoot := isPOSIX && uid == "0"

	// 3. Parsing command line arguments.
	var ourArgs, taskArgs []string
	if i := indexOf(args, "--"); i != -1 {
		ourArgs = args[:i]
		taskArgs = args[i+1:]
	} else {
		ourArgs = args
		taskArgs = []string{}
	}

	// 4. Argument parsing and JSON.
	repositoriesStr := os.Getenv("REPOSITORIES")
	var repositories map[string]string
	if repositoriesStr == "" {
		repositories = map[string]string{"vcs": "repository"}
	} else {
		json.Unmarshal([]byte(repositoriesStr), &repositories)
	}

	user := flag.String("user", "worker", "user to run as")
	group := flag.String("group", "worker", "group to run as")
	taskCwd := flag.String("task-cwd", "", "directory to run the provided command in")
	fetchHgFingerprint := flag.Bool("fetch-hgfingerprint", false, "hidden help")
	var vcsCheckouts = make(map[string]*string)
	var vcsSparseProfiles = make(map[string]*string)

	for repository, name := range repositories {
		addVcsArguments(vcsCheckouts, vcsSparseProfiles, repository, name)
	}

	flag.Parse()

	// 6. Collecting vcs options.
	var vcsOptions []VcsOption
	for repository, name := range repositories {
		vcsOption := collectVcsOptions(*user, repository, name)
		vcsOptions = append(vcsOptions, vcsOption)
	}
	fmt.Println("")
	fmt.Println(runningAsRoot)
	fmt.Println(ourArgs)
	fmt.Println(taskArgs)
	fmt.Println(group)
	fmt.Println(taskCwd)
	fmt.Println(fetchHgFingerprint)
}

func indexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}
