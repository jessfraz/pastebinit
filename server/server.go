package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/sourcegraph/syntaxhighlight"
)

const (
	htmlBegin string = `<!DOCTYPE html>
<html lang="en-US">
<head>
<meta charset="UTF-8">
<link rel="shortcut icon" href="/static/favicon.ico" />
<link rel="stylesheet" media="all" href="/static/main.css"/>
</head>
<body>`
	htmlEnd string = `</body>
</html>`
)

var (
	baseuri  string
	port     string
	storage  string
	certFile string
	keyFile  string

	username = os.Getenv("PASTEBINIT_USERNAME")
	password = os.Getenv("PASTEBINIT_PASS")
)

// JSONResponse is a map[string]string
// response from the web server.
type JSONResponse map[string]string

// String returns the string representation of the
// JSONResponse object.
func (j JSONResponse) String() string {
	str, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{
  "error": "%v"
}`, err)
	}

	return string(str)
}

// generateIndexHTML generates the html for the index page
// to list all the pastes, via walking the storage directory.
func generateIndexHTML() (string, error) {
	var files string

	// create the function to walk the pastes files
	walkPastes := func(pth string, f os.FileInfo, err error) error {
		base := filepath.Base(pth)
		if base != storage {
			files += fmt.Sprintf(`<tr>
<td><a href="%s%s">%s</a></td>
<td>%s</td>
<td>%d</td>
</tr>`, baseuri, base, base, f.ModTime().Format("2006-01-02T15:04:05Z07:00"), f.Size())
		}
		return nil
	}

	// walk the pastes
	if err := filepath.Walk(storage, walkPastes); err != nil {
		return "", fmt.Errorf("walking %s failed: %v", storage, err)
	}

	html := fmt.Sprintf(`%s
<table>
	<thead>
		<tr>
			<th>name</th><th>modified</th><th>size</th>
		</tr>
	</thead>
	<tbody>
		%s
	</tbody>
</table>
%s`, htmlBegin, files, htmlEnd)

	return html, nil
}

// pasteHandler is the request handler for / and /{pasteid}
// it returns a list of all pastes to / if properly authed
// and returns the paste to the public if /{pasteid} exists.
func pasteHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		// they want the root, make them auth
		u, p, ok := r.BasicAuth()
		if (u != username || p != password) || !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+baseuri+`"`)
			w.WriteHeader(401)
			w.Write([]byte("401 Unauthorized\n"))
			return
		}

		html, err := generateIndexHTML()
		if err != nil {
			writeError(w, fmt.Sprintf("generating index html failed: %v", err))
			return
		}

		// write the html
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, html)
		logrus.Info("index file rendered")
		return
	}

	filename := filepath.Join(storage, strings.Trim(r.URL.Path, "/"))
	raw := strings.HasSuffix(filename, "/raw")

	if raw {
		// if they want the raw file serve a text/plain Content-Type
		w.Header().Set("Content-Type", "text/plain")
		// trim '/raw' from the filename so we can get the right file
		filename = strings.TrimSuffix(filename, "/raw")
	}

	// check if they want html
	if strings.HasSuffix(filename, "/html") {
		w.Header().Set("Content-Type", "text/html")
		filename = strings.TrimSuffix(filename, "/html")
		raw = true
	}

	// check if the file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		writeError(w, fmt.Sprintf("No such file or directory: %s", filename))
		return
	}

	// read the file
	src, err := ioutil.ReadFile(filename)
	if err != nil {
		writeError(w, fmt.Sprintf("Reading file %s failed: %v", filename, err))
		return
	}

	if raw {
		// serve the raw file
		w.Write(src)
		logrus.Printf("raw paste served: %s", filename)
		return
	}

	// try to syntax highlight the file
	highlighted, err := syntaxhighlight.AsHTML(src)
	if err != nil {
		writeError(w, fmt.Sprintf("Highlighting file %s failed: %v", filename, err))
		return
	}

	// serve the highlighted file
	fmt.Fprintf(w, "%s<pre><code>%s</code></pre>%s", htmlBegin, string(highlighted), htmlEnd)
	logrus.Printf("highlighted paste served: %s", filename)
	return
}

// pasteUploadHander is the request handler for /paste
// it creates a uuid for the paste and saves the contents of
// the paste to that file.
func pasteUploadHandler(w http.ResponseWriter, r *http.Request) {
	// check basic auth
	u, p, ok := r.BasicAuth()
	if (u != username || p != password) || !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="`+baseuri+`"`)
		w.WriteHeader(401)
		w.Write([]byte("401 Unauthorized\n"))
		return
	}

	// set the content type and check to make sure they are POST-ing a paste
	w.Header().Set("Content-Type", "application/json")
	if r.Method != "POST" {
		writeError(w, "not a valid endpoint")
		return
	}

	// read the body of the paste
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		writeError(w, fmt.Sprintf("reading from body failed: %v", err))
		return
	}

	// create a unique id for the paste
	id, err := uuid()
	if err != nil {
		writeError(w, fmt.Sprintf("uuid generation failed: %v", err))
		return
	}

	// write to file
	file := filepath.Join(storage, id)
	if err := ioutil.WriteFile(file, content, 0755); err != nil {
		writeError(w, fmt.Sprintf("writing file to %q failed: %v", file, err))
		return
	}

	// serve the uri for the paste to the requester
	fmt.Fprint(w, JSONResponse{
		"uri": baseuri + id,
	})
	logrus.Infof("paste %q posted successfully", id)
	return
}

// uuid generates a uuid for the paste.
// This really does not need to be perfect.
func uuid() (string, error) {
	var chars = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")

	length := 8
	b := make([]byte, length)
	r := make([]byte, length+(length/4))
	maxrb := 256 - (256 % len(chars))
	i := 0
	for {
		if _, err := io.ReadFull(rand.Reader, r); err != nil {
			return "", err
		}
		for _, rb := range r {
			c := int(rb)
			if c > maxrb {
				continue
			}
			b[i] = chars[c%len(chars)]
			i++
			if i == length {
				return string(b), nil
			}
		}
	}
}

// writeError sends an error back to the requester
// and also logs the error.
func writeError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, JSONResponse{
		"error": msg,
	})
	logrus.Printf("writing error: %s", msg)
	return
}

func init() {
	flag.StringVar(&baseuri, "b", "https://paste.j3ss.co/", "url base for this domain")
	flag.StringVar(&port, "p", "8080", "port for server to run on")
	flag.StringVar(&storage, "s", "/etc/pastebinit/files", "directory to store pastes")
	flag.StringVar(&certFile, "-cert", "", "path to ssl certificate")
	flag.StringVar(&keyFile, "-key", "", "path to ssl key")
	flag.Parse()

	if username == "" || password == "" {
		logrus.Fatalf("you need to pass the PASTEBINIT_USERNAME and PASTEBINIT_PASS env variables")
	}

	// ensure uri has trailing slash
	if !strings.HasSuffix(baseuri, "/") {
		baseuri += "/"
	}
}

func main() {
	// create the storage directory
	if err := os.MkdirAll(storage, 0755); err != nil {
		logrus.Fatalf("creating storage directory %q failed: %v", storage, err)
	}

	// create mux server
	mux := http.NewServeMux()

	// static files handler
	staticHandler := http.StripPrefix("/static/", http.FileServer(http.Dir("/src/static")))
	mux.Handle("/static/", staticHandler)

	// pastes & view handlers
	mux.HandleFunc("/paste", pasteUploadHandler) // paste upload handler
	mux.HandleFunc("/", pasteHandler)            // index & paste server handler

	// set up the server
	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}
	logrus.Infof("Starting server on port %q with baseuri %q", port, baseuri)
	if certFile != "" && keyFile != "" {
		logrus.Fatal(server.ListenAndServeTLS(certFile, keyFile))
	} else {
		logrus.Fatal(server.ListenAndServe())
	}
}
