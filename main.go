package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/genuinetools/pkg/cli"
	"github.com/jessfraz/pastebinit/version"
	"github.com/sirupsen/logrus"
)

var (
	baseuri  string
	username string
	password string

	debug bool
)

func main() {
	// Create a new cli program.
	p := cli.NewProgram()
	p.Name = "pastebinit"
	p.Description = "Command line paste bin"
	// Set the GitCommit and Version.
	p.GitCommit = version.GITCOMMIT
	p.Version = version.VERSION

	// Build the list of available commands.
	p.Commands = []cli.Command{
		&serverCommand{},
	}

	// Setup the global flags.
	p.FlagSet = flag.NewFlagSet("global", flag.ExitOnError)
	p.FlagSet.StringVar(&baseuri, "b", "https://paste.j3ss.co/", "pastebin base uri")
	p.FlagSet.StringVar(&baseuri, "uri", "https://paste.j3ss.co/", "pastebin base uri")

	p.FlagSet.StringVar(&username, "u", os.Getenv("PASTEBINIT_USERNAME"), "username (or env var PASTEBINIT_USERNAME)")
	p.FlagSet.StringVar(&username, "username", os.Getenv("PASTEBINIT_USERNAME"), "username (or env var PASTEBINIT_USERNAME)")

	p.FlagSet.StringVar(&password, "p", os.Getenv("PASTEBINIT_PASSWORD"), "password (or env var PASTEBINIT_PASSWORD)")
	p.FlagSet.StringVar(&password, "password", os.Getenv("PASTEBINIT_PASSWORD"), "password (or env var PASTEBINIT_PASSWORD)")

	p.FlagSet.BoolVar(&debug, "d", false, "enable debug logging")
	p.FlagSet.BoolVar(&debug, "debug", false, "enable debug logging")

	// Set the before function.
	p.Before = func(ctx context.Context) error {
		// On ^C, or SIGTERM handle exit.
		signals := make(chan os.Signal, 0)
		signal.Notify(signals, os.Interrupt)
		signal.Notify(signals, syscall.SIGTERM)
		_, cancel := context.WithCancel(ctx)
		go func() {
			for sig := range signals {
				cancel()
				logrus.Infof("Received %s, exiting.", sig.String())
				os.Exit(0)
			}
		}()

		// Set the log level.
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		}

		// make sure uri ends with trailing /
		if !strings.HasSuffix(baseuri, "/") {
			baseuri += "/"
		}

		// make sure it starts with http(s)://
		if !strings.HasPrefix(baseuri, "http") {
			baseuri = "http://" + baseuri
		}

		// make sure we have a username and password
		if len(username) < 1 {
			return errors.New("username cannot be empty")
		}
		if len(password) < 1 {
			return errors.New("password cannot be empty")
		}

		return nil
	}

	p.Action = func(ctx context.Context, args []string) error {
		// check if we are reading from a file or stdin
		var content []byte
		if len(args) == 0 {
			content = readFromStdin()
		} else {
			filename := args[0]
			content = readFromFile(filename)
		}

		pasteURI, err := postPaste(content)
		if err != nil {
			return err
		}

		fmt.Printf("Your paste has been uploaded here:\n%s\nthe raw object is here: %s/raw", pasteURI, pasteURI)
		return nil
	}

	// Run our program.
	p.Run()
}

// readFromStdin returns everything in stdin.
func readFromStdin() []byte {
	stdin, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		logrus.Fatalf("reading from stdin failed: %v", err)
	}
	return stdin
}

// readFromFile returns the contents of a file.
func readFromFile(filename string) []byte {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		logrus.Fatalf("No such file or directory: %q", filename)
	}

	file, err := ioutil.ReadFile(filename)
	if err != nil {
		logrus.Fatalf("reading from file %q failed: %v", filename, err)
	}
	return file
}

// postPaste uploads the paste content to the server
// and returns the paste URI.
func postPaste(content []byte) (string, error) {
	// create the request
	req, err := http.NewRequest("POST", baseuri+"paste", bytes.NewBuffer(content))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(username, password)

	// do the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request to %spaste failed: %v", baseuri, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return "", fmt.Errorf("Unauthorized. Please check your username and pass. %d", resp.StatusCode)
	}

	if resp.StatusCode == 413 {
		return "", fmt.Errorf("%d: Payload Too Large. Make sure your proxy or load balancer allows request bodies as large as any file you wish to accept", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response body failed: %v", err)
	}

	var response map[string]string
	if err = json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("parsing body as json failed: %v", err)
	}

	if respError, ok := response["error"]; ok {
		return "", fmt.Errorf("server responded with %s", respError)
	}

	pasteURI, ok := response["uri"]
	if !ok {
		return "", fmt.Errorf("what the hell did we get back even? %s", string(body))
	}

	return pasteURI, nil
}
