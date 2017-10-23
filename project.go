package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Project is a project
type Project struct {
	ProjectConfig
	lock sync.Mutex
}

func (project *Project) deploy() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("=======Recover error:==========")
			fmt.Println(err)
		}
	}()

	project.lock.Lock()
	defer project.lock.Unlock()

	debugf("Begin to deploy project: %v", project)

	if project.Exec != "" {
		project.execProgram("sh", "-xc", project.Exec)
	} else if project.Type == "github" {
		project.deployGithubProject()
	} else if project.Type == "gogs" {
		project.deployGogsProject()
	} else {
		log.Fatalf("Unknown project type: %s, project: %#v", project.Type, project)
	}

	debugf("End to deploy project: %v", project)
}

func (project *Project) deployGithubProject() {
	repoURL := project.RepoURL
	if repoURL == "" {
		repoURL = "git@github.com:" + project.Repo
	}

	project.deployGitProject(repoURL)
}

func (project *Project) deployGogsProject() {
	project.deployGitProject("")
}

func (project *Project) deployGitProject(repoURL string) {
	if repoURL == "" {
		repoURL = project.RepoURL
		if repoURL == "" {
			log.Fatalf("Invalid repo URL for project: %#v", project)
		}
	}

	if !directoryExists(project.Target) {
		makeDir(project.Target)
	}

	storeDir := project.Store
	if storeDir == "" {
		storeDir = project.Target + ".store"
	}

	if !directoryExists(storeDir) {
		makeDir(storeDir)
	}

	project.gitCheckoutToDir(repoURL, storeDir, project.Branch)

	if project.PostCheckoutScript != "" {
		project.execProgram("sh", "-xc", fmt.Sprintf("cd '%s' && %s", storeDir, project.PostCheckoutScript))
	}

	project.rsync(storeDir+"/", project.Target)

	if project.PostRsyncScript != "" {
		project.execProgram("sh", "-xc", fmt.Sprintf("cd '%s' && %s", project.Target, project.PostRsyncScript))
	}
}

func (project *Project) execProgram(program string, args ...string) *os.ProcessState {
	debugf("[EXEC] %s %v", program, args)
	exeFile, err := exec.LookPath(program)
	if err != nil {
		exeFile = program
	}

	cmd := exec.Command(exeFile, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if project.RunAs != "" {
		uid, err := parseUid(project.RunAs)
		if err != nil {
			log.Panic(err)
			return nil
		}

		err = setCmdCredential(cmd, int64(uid), 0)
		if err != nil {
			log.Panic(err)
			return nil
		}
	}

	err = cmd.Start()
	if err != nil {
		log.Panic(err)
		return nil
	}

	// log.Print("Command executing...")

	err = cmd.Wait()

	// log.Printf("Command finished with error: %v", err)
	if err != nil {
		// if exitErr, ok := err.(*exec.ExitError); ok {
		// 	// This program has exited with exit code != 0
		// 	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
		// 		os.Exit(status.ExitStatus())
		// 	}
		// }

		panic(err)
	}

	return cmd.ProcessState
}

func (project *Project) gitCheckoutToDir(repoURL, targetDir, branch string) {
	var cmds []string
	if directoryExists(targetDir + "/.git") {
		cmds = []string{
			fmt.Sprintf("cd '%s'", targetDir),
			fmt.Sprintf("git checkout ."),
			fmt.Sprintf("git fetch origin '%s'", branch),
			fmt.Sprintf("git reset --hard origin/'%s'", branch),
		}

	} else {
		cmds = []string{
			fmt.Sprintf("cd '%s'", targetDir),
			fmt.Sprintf("git clone --depth 1 --branch '%s' '%s' .", branch, repoURL),
		}
	}

	project.execProgram("sh", "-xc", strings.Join(cmds, " && "))
}

func (project *Project) rsync(fromDir, toDir string) {
	args := []string{
		"-avzC", "--delay-updates", "--omit-dir-times",
	}

	excludeFromFile := fromDir + "/.rsync_ignores"
	if fileExists(excludeFromFile) {
		args = append(args, "--exclude-from="+excludeFromFile)
	}

	args = append(args, fromDir, toDir)

	project.execProgram("rsync", args...)
}
