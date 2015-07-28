package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

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
	username string = os.Getenv("PASTEBINIT_USERNAME")
	pass     string = os.Getenv("PASTEBINIT_PASS")

	chars = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")
)

func init() {
	flag.StringVar(&baseuri, "b", "https://paste.j3ss.co/", "url base for this domain")
	flag.StringVar(&port, "p", "8080", "port for server to run on")
	flag.StringVar(&storage, "s", "files", "directory to store pastes")
	flag.StringVar(&certFile, "-cert", "", "path to ssl certificate")
	flag.StringVar(&keyFile, "-key", "", "path to ssl key")

	flag.Parse()

	if username == "" || pass == "" {
		log.Fatalf("you need to pass the PASTEBINIT_USERNAME and PASTEBINIT_PASS env variables")
	}

	// ensure uri has trailing slash
	if !strings.HasSuffix(baseuri, "/") {
		baseuri += "/"
	}
}

type JSONResponse map[string]string

func (j JSONResponse) String() string {
	str, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{
  "error": "%v"
}`, err)
	}

	return string(str)
}

func auth(r *http.Request) bool {
	if _, ok := r.Header["Authorization"]; !ok {
		log.Print("no auth header found")
		return false
	}
	auth := strings.SplitN(r.Header["Authorization"][0], " ", 2)

	if len(auth) != 2 || auth[0] != "Basic" {
		log.Print("bad auth syntax")
		return false
	}

	payload, err := base64.StdEncoding.DecodeString(auth[1])
	if err != nil {
		log.Printf("decoding auth string failed: %v", err)
		return false
	}
	pair := strings.SplitN(string(payload), ":", 2)

	if len(pair) != 2 || pair[0] != username || pair[1] != pass {
		log.Printf("someone tried and failed to get in")
		return false
	}

	return true
}

func writeError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, JSONResponse{
		"error": msg,
	})
	log.Printf("writing error: %s", msg)
	return
}

func uuid() (string, error) {
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

	return "", errors.New("[uuid]: We should not have gotten here")
}

func main() {
	// create the storage directory
	if err := os.MkdirAll(storage, 0755); err != nil {
		log.Fatalf("creating storage directory %q failed: %v", storage, err)
	}

	// create mux server
	mux := http.NewServeMux()

	// static files
	staticHandler := http.StripPrefix("/static/", http.FileServer(http.Dir("/src/static")))
	mux.Handle("/static/", staticHandler)

	// upload function
	mux.HandleFunc("/paste", func(w http.ResponseWriter, r *http.Request) {
		// make them auth
		allowed := auth(r)
		if !allowed {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+baseuri+`"`)
			w.WriteHeader(401)
			w.Write([]byte("401 Unauthorized\n"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if r.Method != "POST" {
			writeError(w, "not a valid endpoint")
			return
		}

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
		filepath := path.Join(storage, id)
		if err = ioutil.WriteFile(filepath, content, 0755); err != nil {
			writeError(w, fmt.Sprintf("writing file to %q failed: %v", filepath, err))
			return
		}

		fmt.Fprint(w, JSONResponse{
			"uri": baseuri + id,
		})
		log.Printf("paste posted successfully")
		return
	})

	// index function
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/" {
			// they want the root
			// if probably me the admin
			// but make them auth
			allowed := auth(r)
			if !allowed {
				w.Header().Set("WWW-Authenticate", `Basic realm="`+baseuri+`"`)
				w.WriteHeader(401)
				w.Write([]byte("401 Unauthorized\n"))
				return
			}

			var files string
			err := filepath.Walk(storage, func(path string, f os.FileInfo, err error) error {
				if filepath.Base(path) != storage {
					files += fmt.Sprintf(`<tr>
<td><a href="%s%s">%s</a></td>
<td>%s</td>
<td>%d</td>
</tr>`, baseuri, filepath.Base(path), filepath.Base(path), f.ModTime().Format("2006-01-02T15:04:05Z07:00"), f.Size())
				}
				return nil
			})
			if err != nil {
				writeError(w, fmt.Sprintf("walking %s failed: %v", storage, err))
			}

			fmt.Fprintf(w, "%s<table><thead><tr><th>name</th><th>modified</th><th>size</th></tr></thead><tbody>%s</tbody></table>%s", htmlBegin, files, htmlEnd)
			log.Printf("index file rendered")
			return
		}

		// check if a file exists for what they are asking
		serveRaw := false
		filename := strings.Trim(r.URL.Path, "/")
		filename = path.Join(storage, filename)

		// check if they want the raw file
		if strings.HasSuffix(filename, "/raw") {
			w.Header().Set("Content-Type", "text/plain")
			filename = strings.TrimSuffix(filename, "/raw")
			serveRaw = true
		}
		// check if they want the raw file
		if strings.HasSuffix(filename, "/html") {
			w.Header().Set("Content-Type", "text/html")
			filename = strings.TrimSuffix(filename, "/html")
			serveRaw = true
		}

		// check if file exists
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			writeError(w, fmt.Sprintf("No such file or directory: %q", filename))
			return
		}

		// read the file
		src, err := ioutil.ReadFile(filename)
		if err != nil {
			writeError(w, fmt.Sprintf("Reading file %q failed: %v", filename, err))
			return
		}

		if serveRaw {
			// serve the file
			w.Write(src)
			log.Printf("raw paste served: %s", filename)
			return
		}

		// try to syntax highlight the file
		highlighted, err := syntaxhighlight.AsHTML(src)
		if err != nil {
			writeError(w, fmt.Sprintf("Highlighting file %q failed: %v", filename, err))
			return
		}

		fmt.Fprintf(w, "%s<pre><code>%s</code></pre>%s", htmlBegin, string(highlighted), htmlEnd)
		log.Printf("highlighted paste served: %s", filename)
		return
	})

	// set up the server
	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}
	log.Printf("Starting server on port %q with baseuri %q", port, baseuri)
	if certFile != "" && keyFile != "" {
		log.Fatal(server.ListenAndServeTLS(certFile, keyFile))
	} else {
		log.Fatal(server.ListenAndServe())
	}
}
