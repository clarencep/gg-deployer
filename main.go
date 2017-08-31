package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// Whether to enable debug
var debugEnabled = false

type HttpServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type GGDConfig struct {
	HttpServer HttpServerConfig `json:"http_server"`
}

var ggdConfig GGDConfig
var thisExeFilePath = ""

func init() {
	var err error
	var arg0 = os.Args[0]

	thisExeFilePath, err = exec.LookPath(arg0)
	if err != nil {
		thisExeFilePath = arg0
	}
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

	http.HandleFunc("/", homeHandler)

	addr := ggdConfig.HttpServer.Host + ":" + strconv.Itoa(ggdConfig.HttpServer.Port)
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
		if exitErr, ok := err.(*exec.ExitError); ok {
			// This program has exited with exit code != 0
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}

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

	if ggdConfig.HttpServer.Host == "0.0.0.0" {
		ggdConfig.HttpServer.Host = ""
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	debugf("Enter homeHandler...")

	fmt.Fprintf(w, "<h1>Hello world</h1>")
}
