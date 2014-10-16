package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

var (
	args     []string
	content  []byte
	baseuri  string
	username string = os.Getenv("PASTEBINIT_USERNAME")
	pass     string = os.Getenv("PASTEBINIT_PASS")
)

func init() {
	flag.StringVar(&baseuri, "b", "https://paste.j3ss.co/", "pastebin base url")
	flag.Parse()

	args = flag.Args()

	// make sure uri ends with trailing /
	if !strings.HasSuffix(baseuri, "/") {
		baseuri += "/"
	}

	// make sure it starts with http(s)://
	if !strings.HasPrefix(baseuri, "http") {
		baseuri = "http://" + baseuri
	}

	if username == "" || pass == "" {
		log.Fatalf("you need to pass the PASTEBINIT_USERNAME and PASTEBINIT_PASS env variables")
	}
}

func readFromStdin() []byte {
	stdin, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("reading from stdin failed: %v", err)
	}
	return stdin
}

func readFromFile(filename string) []byte {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Fatalf("No such file or directory: %q", filename)
	}

	file, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("reading from file %q failed: %v", filename, err)
	}
	return file
}

func main() {
	// check if we are reading from a file or stdin
	if len(args) == 0 {
		content = readFromStdin()
	} else {
		filename := args[0]
		content = readFromFile(filename)
	}

	// create the request
	req, err := http.NewRequest("POST", baseuri+"paste", bytes.NewBuffer(content))
	if err != nil {
		log.Fatalf("creating new http request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(username, pass)

	// do the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("request to %spaste failed: %v", baseuri, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		log.Fatal("Unauthorized. Please check your username and pass.")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("reading response body failed: %v", err)
	}

	var response map[string]string
	if err = json.Unmarshal(body, &response); err != nil {
		log.Fatalf("parsing body as json failed: %v", err)
	}

	if respError, ok := response["error"]; ok {
		log.Fatalf("server responded with %s", respError)
	}

	var pasteUri string
	var ok bool
	if pasteUri, ok = response["uri"]; !ok {
		log.Fatalf("what the hell did we get back even? %s", string(body))
	}

	fmt.Printf("Your paste has been uploaded here:\n%s\nthe raw object is here: %s/raw", pasteUri, pasteUri)
}
