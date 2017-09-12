package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Whether to enable debug
var debugEnabled = false

// HTTPServerConfig is configuration for HTTP server
type HTTPServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// ProjectConfig is configuration for a project
type ProjectConfig struct {
	Type               string `json:"type"`
	Repo               string `json:"repo"`
	RepoURL            string `json:"repo_url"`
	Secret             string `json:"secret"`
	Store              string `json:"store"`
	Target             string `json:"target"`
	Branch             string `json:"branch"`
	PostCheckoutScript string `json:"post_checkout_script"`
	PostRsyncScript    string `json:"post_rsync_script"`
}

// Project is a project
type Project struct {
	ProjectConfig
	lock sync.Mutex
}

// GGDConfig is the configuration for gg-deployer
type GGDConfig struct {
	HTTPServer HTTPServerConfig `json:"http_server"`
	Projects   []ProjectConfig  `json:"projects"`
	MaxWorkers int              `json:"max_workers"`
}

// DeployJob is a job for deploy
type DeployJob struct {
	Project   *Project
	StartTime time.Time
}

// HTTPError is an error with HTTP status
type HTTPError struct {
	Status  int
	Message string
}

var ggdConfig GGDConfig
var allProjects []*Project

var thisExeFilePath = ""
var deployJobsQueue chan *DeployJob

func init() {
	var err error
	var arg0 = os.Args[0]

	// 初始化当前EXE文件的路径
	thisExeFilePath, err = exec.LookPath(arg0)
	if err != nil {
		thisExeFilePath = arg0
	}

	// 初始化队列
	deployJobsQueue = make(chan *DeployJob, 100)
}

func main() {
	args := enableDebugModeIfNeeded(os.Args)

	debugf("args: %v\n", args)

	if args[1] != "-c" {
		printHelp()
		return
	}

	loadConfig(args[2])

	debugf("loaded config: %v", ggdConfig)

	// Start the job queue
	for i := 0; i < ggdConfig.MaxWorkers; i++ {
		go processDeployJobs()
	}

	// Start the HTTP server
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/test", testHandler)
	http.HandleFunc("/github/pushed", githubPushedHandler)
	http.HandleFunc("/gogs/pushed", gogsPushedHandler)
	http.HandleFunc("/list-jobs", listJobsHandler)

	addr := ggdConfig.HTTPServer.Host + ":" + strconv.Itoa(ggdConfig.HTTPServer.Port)
	debugf("Try to serve at " + addr)

	log.Fatal(http.ListenAndServe(addr, nil))

	return
}

func debugf(fmt string, v ...interface{}) {
	if debugEnabled {
		v = append([]interface{}{os.Getpid()}, v...)
		log.Printf("[%v] "+fmt, v...)
	}
}

func enableDebugModeIfNeeded(args []string) []string {
	for i, n := 1, len(args); i < n; i++ {
		if args[i] == "--debug" {
			debugEnabled = true
			return append(append([]string{}, args[0:i]...), args[i+1:]...)
		}
	}

	return args
}

func printHelp() {
	fmt.Println(`Usages:
gg-deployer -c <config-file>
`)
	os.Exit(1)
}

func execProgram(program string, args ...string) *os.ProcessState {
	debugf("[EXEC] %s %v", program, args)
	exeFile, err := exec.LookPath(program)
	if err != nil {
		exeFile = program
	}

	cmd := exec.Command(exeFile, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

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

func getFileContents(filepath string) []byte {
	content, err := ioutil.ReadFile(filepath)
	if err != nil {
		panic(err)
	}

	return content
}

func loadConfig(configFilePath string) {
	configFileContent := getFileContents(configFilePath)

	if err := json.Unmarshal(configFileContent, &ggdConfig); err != nil {
		panic(err)
	}

	if ggdConfig.HTTPServer.Host == "0.0.0.0" {
		ggdConfig.HTTPServer.Host = ""
	}

	if ggdConfig.HTTPServer.Port <= 0 {
		ggdConfig.HTTPServer.Port = 8088
	}

	if ggdConfig.MaxWorkers <= 0 {
		ggdConfig.MaxWorkers = 1
	}

	allProjects = make([]*Project, len(ggdConfig.Projects))
	for i, projectConfig := range ggdConfig.Projects {
		if (projectConfig.Type != "github") && (projectConfig.Type != "gogs") {
			panic(fmt.Errorf("Unknown project type: %#v", projectConfig))
		}

		if projectConfig.Repo == "" {
			panic(fmt.Errorf("Invalid project repo: %#v", projectConfig))
		}

		if projectConfig.RepoURL == "" && projectConfig.Type != "github" {
			panic(fmt.Errorf("Invalid project repo_url: %#v", projectConfig))
		}

		if projectConfig.Secret == "" {
			panic(fmt.Errorf("Invalid project secret: %#v", projectConfig))
		}

		if projectConfig.Target == "" {
			panic(fmt.Errorf("Invalid project target: %#v", projectConfig))
		}

		if projectConfig.Branch == "" {
			panic(fmt.Errorf("Invalid project branch: %#v", projectConfig))
		}

		allProjects[i] = &Project{
			ProjectConfig: projectConfig,
		}
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	debugf("Enter homeHandler...")

	fmt.Fprintf(w, "<h1>Hello world</h1>")

}

func testHandler(w http.ResponseWriter, r *http.Request) {
	debugf("Enter testHandler...")

	fmt.Fprintf(w, "<h1>Test</h1>")
	fmt.Fprintf(w, "<p>URL: %v</p>\n", r.URL)
	fmt.Fprintf(w, "<p>Query: %v</p>\n", r.URL.Query()) // 注意，Query()返回的是一个 string => []string 的结构
	fmt.Fprintf(w, "<p></p>\n")
	fmt.Fprintf(w, "<p>Form before parse: %v</p>\n", r.Form)

	// 注意：ParseForm()会把POST和GET的数据都填充到 r.Form 中去，POST的数据在前面，GET的在后面
	err := r.ParseForm()
	if err != nil {
		fmt.Fprintf(w, "<p>Form parse failed: %v</p>\n", err)
	} else {
		fmt.Fprintf(w, "<p>Parsed form: %v</p>\n", r.Form)
	}

	fmt.Fprintf(w, "<p>Header: %v</p>\n", r.Header)

	var jsonData interface{}
	err = parseHTTPRequestBodyAsJSON(r, &jsonData)
	if err != nil {
		fmt.Fprintf(w, "<p>JSON content parse failed: %v</p>\n", err)
	} else {
		fmt.Fprintf(w, "<p>JSON content parsed: %v</p>\n", jsonData)
		fmt.Fprintf(w, "<p> json.a: %v</p>\n", jsonGet(jsonData, "a"))
		fmt.Fprintf(w, "<p> json.a.b: %v</p>\n", jsonGet(jsonData, "a.b"))
		fmt.Fprintf(w, "<p> json.a.b.c: %v</p>\n", jsonGet(jsonData, "a.b.c"))
		fmt.Fprintf(w, "<p> json.b.1: %v</p>\n", jsonGet(jsonData, "b.1"))
	}
}

func listJobsHandler(w http.ResponseWriter, r *http.Request) {
	debugf("Enter listJobsHandler...")
	fmt.Fprintf(w, "<p> %d jobs: %#v</p>\n", len(deployJobsQueue), deployJobsQueue)
}

func githubPushedHandler(w http.ResponseWriter, r *http.Request) {
	debugf("Enter githubPushedHandler...")
	debugf("request headers: %v", r.Header)

	err := func() error {
		if r.Method != "POST" {
			return &HTTPError{Message: "Error: invalid request method."}
		}

		reqSign := firstOf(r.Header["X-Hub-Signature"])
		if reqSign == "" {
			debugf("Error: no signature in request.")
			return &HTTPError{Message: "Error: no signature in request."}
		}

		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			debugf("Error: failed to read request body: %v", err)
			return &HTTPError{Message: "Error: Empty request body"}
		}

		// debugf("request body: %s", string(reqBody))

		var jsonData interface{}
		err = json.Unmarshal(reqBody, &jsonData)
		if err != nil {
			debugf("Error: Failed to parse JSON. Error: %v", err)
			return &HTTPError{Message: "Invalid JSON"}
		}

		// debugf("parsed json: %v", jsonData)

		// queryRepo := firstOf(r.URL.Query()["repo"])
		// if queryRepo == "" {
		// 	debugf("Error: Failed to get repo from query string")
		// 	return &HTTPError{Message: "Invalid query repo"}
		// }

		postRepo, ok := jsonGet(jsonData, "repository.full_name").(string)
		if !ok || postRepo == "" {
			debugf("Error: Failed to get repo from post payload")
			return &HTTPError{Message: "Invalid post repo"}
		}

		// if queryRepo != postRepo {
		// 	debugf("Error: queryRepo(%s) not match postRepo(%s)", queryRepo, postRepo)
		// 	return &HTTPError{Message: "Invalid repo"}
		// }

		reqRepo := postRepo

		foundProjects := findProjectsByRepo(reqRepo, "github")
		debugf("Info: found %d projects by repo %s: %#v", len(foundProjects), reqRepo, foundProjects)

		projectsResult := make([]string, len(foundProjects))

		for i, project := range foundProjects {
			if verifyGithubSignature(reqBody, reqSign, project.Secret) {
				err = queueDeployJob(&DeployJob{
					Project:   project,
					StartTime: time.Now(),
				})

				if err != nil {
					projectsResult[i] = err.Error()
				} else {
					projectsResult[i] = "OK"
				}
			} else {
				debugf("Warn: failed to verify github signature for %s", reqRepo)
				projectsResult[i] = "Error: failed to verify signature"
			}

		}

		responseBuf, err := json.Marshal(map[string]interface{}{
			"projects": projectsResult,
		})

		if err != nil {
			return err
		}

		w.Write(responseBuf)

		return nil
	}()

	if err != nil {
		httpError, ok := err.(*HTTPError)
		if ok && httpError.Status > 0 {
			w.WriteHeader(httpError.Status)
		} else {
			w.WriteHeader(http.StatusNotAcceptable)
		}

		fmt.Fprintf(w, err.Error())
	}

	debugf("Leave githubPushedHandler...")
}

func gogsPushedHandler(w http.ResponseWriter, r *http.Request) {
	debugf("Enter gogsPushedHandler...")
	debugf("request headers: %v", r.Header)

	err := func() error {
		if r.Method != "POST" {
			return &HTTPError{Message: "Error: invalid request method."}
		}

		reqSign := firstOf(r.Header["X-Gogs-Signature"])
		if reqSign == "" {
			debugf("Error: no signature in request.")
			return &HTTPError{Message: "Error: no signature in request."}
		}

		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			debugf("Error: failed to read request body: %v", err)
			return &HTTPError{Message: "Error: Empty request body"}
		}

		// debugf("request body: %s", string(reqBody))

		var jsonData interface{}
		err = json.Unmarshal(reqBody, &jsonData)
		if err != nil {
			debugf("Error: Failed to parse JSON. Error: %v", err)
			return &HTTPError{Message: "Invalid JSON"}
		}

		// debugf("parsed json: %v", jsonData)

		postRepo, ok := jsonGet(jsonData, "repository.full_name").(string)
		if !ok || postRepo == "" {
			debugf("Error: Failed to get repo from post payload")
			return &HTTPError{Message: "Invalid post repo"}
		}

		reqRepo := postRepo

		foundProjects := findProjectsByRepo(reqRepo, "gogs")
		debugf("Info: found %d projects by repo %s: %#v", len(foundProjects), reqRepo, foundProjects)

		projectsResult := make([]string, len(foundProjects))

		for i, project := range foundProjects {
			if verifyGogsSignature(reqBody, reqSign, project.Secret) {
				err = queueDeployJob(&DeployJob{
					Project:   project,
					StartTime: time.Now(),
				})

				if err != nil {
					projectsResult[i] = err.Error()
				} else {
					projectsResult[i] = "OK"
				}
			} else {
				debugf("Warn: failed to verify github signature for %s", reqRepo)
				projectsResult[i] = "Error: failed to verify signature"
			}

		}

		responseBuf, err := json.Marshal(map[string]interface{}{
			"projects": projectsResult,
		})

		if err != nil {
			return err
		}

		w.Write(responseBuf)

		return nil
	}()

	if err != nil {
		httpError, ok := err.(*HTTPError)
		if ok && httpError.Status > 0 {
			w.WriteHeader(httpError.Status)
		} else {
			w.WriteHeader(http.StatusNotAcceptable)
		}

		fmt.Fprintf(w, err.Error())
	}

	debugf("Leave gogsPushedHandler...")
}

func parseHTTPRequestBodyAsJSON(r *http.Request, v *interface{}) error {
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	debugf("got buf: %v", buf)
	debugf("buf as string: %s", string(buf))

	err = json.Unmarshal(buf, v)
	if err != nil {
		return err
	}

	debugf("decoded json: %v", *v)

	return nil
}

func jsonGet(data interface{}, path string) interface{} {
	next := data
	pathArr := strings.Split(path, ".")

	for _, v := range pathArr {
		if next != nil {
			switch x := next.(type) {
			case map[string]interface{}: // JSON Object
				next = x[v]

			case []interface{}: // JSON Array
				i, err := strconv.Atoi(v)
				if err != nil {
					return nil
				}

				next = x[i]

			case string: // JSON string
				i, err := strconv.Atoi(v)
				if err != nil {
					return nil
				}

				next = x[i]
			default:
				next = nil
			}
		} else {
			return nil
		}

	}

	return next
}

func firstOf(arr []string) string {
	if arr != nil && len(arr) > 0 {
		return arr[0]
	}

	return ""
}

func findProjectsByRepo(repo, repoType string) []*Project {
	result := make([]*Project, 0)

	for _, project := range allProjects {
		if project.Repo == repo && project.Type == repoType {
			result = append(result, project)
		}
	}

	return result
}

func queueDeployJob(job *DeployJob) error {
	select {
	case deployJobsQueue <- job:
		return nil
	case <-time.After(500 * time.Millisecond):
		return errors.New("Failed to queue job -- timeout")
	}
}

func processDeployJobs() {
	for {
		job := <-deployJobsQueue

		debugf("try to process deploy job: %v", job)
		job.Project.deploy()
	}
}

func verifyGithubSignature(reqBody []byte, reqSign string, secret string) bool {
	// debugf("========reqBody======\n%s", string(reqBody))
	// debugf("========reqSign======\n%s", string(reqSign))
	// debugf("========secret======\n%s", string(secret))

	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(reqBody)
	calcSign := "sha1=" + hex.EncodeToString(mac.Sum([]byte(nil)))

	// debugf("========calcSign======\n%s", string(calcSign))

	return hmac.Equal([]byte(calcSign), []byte(reqSign))
}

func verifyGogsSignature(reqBody []byte, reqSign string, secret string) bool {
	debugf("========reqBody======\n%s", string(reqBody))
	debugf("========reqSign======\n%s", string(reqSign))
	debugf("========secret======\n%s", string(secret))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(reqBody)
	calcSign := hex.EncodeToString(mac.Sum([]byte(nil)))

	debugf("========calcSign======\n%s", string(calcSign))

	return hmac.Equal([]byte(calcSign), []byte(reqSign))
}

func (httpError *HTTPError) Error() string {
	return httpError.Message
}

func (project *Project) deploy() {
	defer func() {
		if err:= recover(); err != nil {
			fmt.Println("=======Recover error:==========")
			fmt.Println(err)
		}
	}()

	project.lock.Lock()
	defer project.lock.Unlock()

	debugf("Begin to deploy project: %v", project)

	if project.Type == "github" {
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

	gitCheckoutToDir(repoURL, storeDir, project.Branch)

	if project.PostCheckoutScript != "" {
		execProgram("sh", "-xc", fmt.Sprintf("cd '%s' && %s", storeDir, project.PostCheckoutScript))
	}

	rsync(storeDir+"/", project.Target)

	if project.PostRsyncScript != "" {
		execProgram("sh", "-xc", fmt.Sprintf("cd '%s' && %s", project.Target, project.PostRsyncScript))
	}
}

func directoryExists(dir string) bool {
	return fileExists(dir)
}

func fileExists(file string) bool {
	_, err := os.Stat(file)
	if err == nil {
		return true
	}

	if os.IsNotExist(err) {
		return false
	}

	return true
}

func makeDir(dir string) {
	os.MkdirAll(dir, 0775)
}

func gitCheckoutToDir(repoURL, targetDir, branch string) {
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

	execProgram("sh", "-xc", strings.Join(cmds, " && "))
}

func rsync(fromDir, toDir string) {
	args := []string{
		"-avzC", "--delay-updates", "--omit-dir-times",
	}

	excludeFromFile := fromDir + "/.rsync_ignores"
	if fileExists(excludeFromFile) {
		args = append(args, "--exclude-from="+excludeFromFile)
	}

	args = append(args, fromDir, toDir)

	execProgram("rsync", args...)
}
