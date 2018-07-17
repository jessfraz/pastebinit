package main

import (
	"context"
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

	"github.com/buildkite/terminal"
	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/syntaxhighlight"
)

const (
	htmlBegin string = `<!DOCTYPE html>
<html lang="en-US">
<head>
<meta charset="UTF-8">
<link rel="shortcut icon" href="/static/favicon.ico" />
<link rel="stylesheet" media="all" href="/static/main.css"/>
<link rel="stylesheet" media="all" href="/static/ansi.css"/>
</head>
<body>`
	htmlEnd string = `</body>
</html>`

	serverHelp = `Run the server.`
)

func (cmd *serverCommand) Name() string      { return "server" }
func (cmd *serverCommand) Args() string      { return "[OPTIONS]" }
func (cmd *serverCommand) ShortHelp() string { return serverHelp }
func (cmd *serverCommand) LongHelp() string  { return serverHelp }
func (cmd *serverCommand) Hidden() bool      { return false }

func (cmd *serverCommand) Register(fs *flag.FlagSet) {
	fs.StringVar(&cmd.cert, "cert", "", "path to ssl cert")
	fs.StringVar(&cmd.key, "key", "", "path to ssl key")
	fs.StringVar(&cmd.port, "port", "8080", "port for server to run on")

	fs.StringVar(&cmd.storage, "s", "/etc/pastebinit/files", "directory to store pastes")
	fs.StringVar(&cmd.storage, "storage", "/etc/pastebinit/files", "directory to store pastes")

	fs.StringVar(&cmd.assetPath, "asset-path", "/src/static", "Path to assets and templates")
}

type serverCommand struct {
	cert string
	key  string
	port string

	storage   string
	assetPath string
}

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

func (cmd *serverCommand) Run(ctx context.Context, args []string) error {
	// create the storage directory
	if err := os.MkdirAll(cmd.storage, 0755); err != nil {
		logrus.Fatalf("creating storage directory %q failed: %v", cmd.storage, err)
	}

	// create mux server
	mux := http.NewServeMux()

	// static files handler
	staticHandler := http.StripPrefix("/static/", http.FileServer(http.Dir(cmd.assetPath)))
	mux.Handle("/static/", staticHandler)

	// pastes & view handlers
	mux.HandleFunc("/paste", cmd.pasteUploadHandler) // paste upload handler
	mux.HandleFunc("/", cmd.pasteHandler)            // index & paste server handler

	// Set up the server.
	server := &http.Server{
		Addr:    ":" + cmd.port,
		Handler: mux,
	}
	logrus.Infof("Starting server on port %d", cmd.port)
	if len(cmd.cert) > 0 && len(cmd.key) > 0 {
		return server.ListenAndServeTLS(cmd.cert, cmd.key)
	}
	return server.ListenAndServe()
}

// generateIndexHTML generates the html for the index page
// to list all the pastes, via walking the storage directory.
func (cmd *serverCommand) generateIndexHTML() (string, error) {
	var files string

	// create the function to walk the pastes files
	walkPastes := func(pth string, f os.FileInfo, err error) error {
		base := filepath.Base(pth)
		if base != cmd.storage {
			files += fmt.Sprintf(`<tr>
<td><a href="%s%s">%s</a></td>
<td>%s</td>
<td>%d</td>
</tr>`, baseuri, base, base, f.ModTime().Format("2006-01-02T15:04:05Z07:00"), f.Size())
		}
		return nil
	}

	// walk the pastes
	if err := filepath.Walk(cmd.storage, walkPastes); err != nil {
		return "", fmt.Errorf("walking %s failed: %v", cmd.storage, err)
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
func (cmd *serverCommand) pasteHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		// they want the root, make them auth
		u, p, ok := r.BasicAuth()
		if (u != username || p != password) || !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+baseuri+`"`)
			w.WriteHeader(401)
			w.Write([]byte("401 Unauthorized\n"))
			return
		}

		html, err := cmd.generateIndexHTML()
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

	filename := filepath.Join(cmd.storage, strings.Trim(r.URL.Path, "/"))

	var handler func(data []byte) (string, error)

	if strings.HasSuffix(filename, "/raw") {
		// if they want the raw file serve a text/plain Content-Type
		w.Header().Set("Content-Type", "text/plain")
		// trim '/raw' from the filename so we can get the right file
		filename = strings.TrimSuffix(filename, "/raw")
		handler = func(data []byte) (string, error) {
			return string(data), nil
		}
	} else if strings.HasSuffix(filename, "/html") {
		// check if they want html
		w.Header().Set("Content-Type", "text/html")
		filename = strings.TrimSuffix(filename, "/html")
		handler = func(data []byte) (string, error) {
			return string(data), nil
		}
	} else if strings.HasSuffix(filename, "/ansi") {
		// check if they want ansi colored text
		w.Header().Set("Content-Type", "text/html")
		filename = strings.TrimSuffix(filename, "/ansi")
		// try to syntax highlight the file
		handler = func(data []byte) (string, error) {
			return fmt.Sprintf("%s<pre><code>%s</code></pre>%s", htmlBegin, terminal.Render(data), htmlEnd), nil
		}
	} else {
		// check if they want html
		w.Header().Set("Content-Type", "text/html")
		handler = func(data []byte) (string, error) {
			highlighted, err := syntaxhighlight.AsHTML(data)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s<pre><code>%s</code></pre>%s", htmlBegin, string(highlighted), htmlEnd), nil
		}
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

	data, err := handler(src)
	if err != nil {
		writeError(w, fmt.Sprintf("Processing file %s failed: %v", filename, err))
	}

	io.WriteString(w, data)
	return
}

// pasteUploadHander is the request handler for /paste
// it creates a uuid for the paste and saves the contents of
// the paste to that file.
func (cmd *serverCommand) pasteUploadHandler(w http.ResponseWriter, r *http.Request) {
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
	file := filepath.Join(cmd.storage, id)
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
