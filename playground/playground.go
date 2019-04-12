// Copyright 2013 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package playground registers HTTP handlers at "/compile" and "/share" that
// proxy requests to the golang.org playground service.
// This package may be used unaltered on App Engine Standard with Go 1.11+ runtime.
package playground // import "golang.org/x/tools/playground"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/tools/godoc/golangorgenv"
)

const baseURL = "https://play.golang.org"

func init() {
	http.HandleFunc("/compile", bounce)
	http.HandleFunc("/share", bounce)
	http.HandleFunc("/gitpull", handleGitPull)
}

type PlayEvent struct {
	Message string
	Kind string
	Delay int
}

type PlayResp struct {
	Errors string
	Events []PlayEvent
	Status int
	IsTest bool
	TestsFailed int
}

func bounce(w http.ResponseWriter, r *http.Request) {
	b := new(bytes.Buffer)
	b.ReadFrom(r.Body)
	s := b.String()[len("version=2&body="):]
	s, err := url.QueryUnescape(s)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	f, err := ioutil.TempFile("", "*.go")
	if err != nil {
		fmt.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println("create temp file:", f.Name())
	f.WriteString(s)
	f.Close()
	defer os.Remove(f.Name())

	out := new(bytes.Buffer)
	cmd := exec.Command("go", "run", f.Name())
	//cmd.
	cmd.Stdout = out
	cmd.Stderr = out
	err = cmd.Run()
	strOut := out.String()
	if err != nil {
		fmt.Println(err, strOut)
		http.Error(w, err.Error() + ":" + strOut, http.StatusInternalServerError)
		return
	}

	resp := PlayResp{
		Events: []PlayEvent{
			{
				Message: out.String(),
				Kind: "stdout",
			},
		},
	}
	json.NewEncoder(w).Encode(resp)
	//if err := passThru(b, r); os.IsPermission(err) {
	//	http.Error(w, "403 Forbidden", http.StatusForbidden)
	//	log.Println(err)
	//	return
	//} else if err != nil {
	//	http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
	//	log.Println(err)
	//	return
	//}
	//io.Copy(w, b)
}

func gitPull(dirPath string, w io.Writer) error {
	cmd := exec.Command("git", "pull")
	cmd.Dir = dirPath
	// w.Write([]byte("cd "))
	// w.Write([]byte(dirPath))
	// w.Write([]byte("\ngit pull\n"))
	log.Println("cd", dirPath)
	log.Println(cmd.Args)
	cmd.Stdout = w
	// cmd.Stderr = os.Stderr
	return cmd.Run()
}

// users/weliu/repos/go_talks/browse
func getRepo(req string) string {
	req = strings.Trim(req, "/")
	const repo = "users/"
	if !strings.HasPrefix(req, repo) {
		return ""
	}
	cnt := 1
	var i int
	for i = len(repo); i < len(req); i++ {
		if req[i] == '/' {
			cnt++
			if cnt == 4 {
				break
			}
		}
	}
	switch cnt {
	case 3:
		return req + "/browse"
	case 4:
		return req[:i] + "/browse"
	default:
		return ""
	}
}

func handleGitPull(w http.ResponseWriter, r *http.Request) {
	b := new(bytes.Buffer)
	b.ReadFrom(r.Body)
	s, err := url.QueryUnescape(b.String())
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dir := getRepo(s)
	if dir == "" {
		return
	}
	o := new(bytes.Buffer)
	err = gitPull(dir, o)
	out := o.String()
	if err != nil {
		log.Println(err, o)
		http.Error(w, err.Error() + "\n" + out, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(out)
}

func passThru(w io.Writer, req *http.Request) error {
	if req.URL.Path == "/share" && googleCN(req) {
		return os.ErrPermission
	}
	defer req.Body.Close()
	url := baseURL + req.URL.Path
	ctx, cancel := context.WithTimeout(req.Context(), 60*time.Second)
	defer cancel()
	r, err := post(ctx, url, req.Header.Get("Content-Type"), req.Body)
	if err != nil {
		return fmt.Errorf("making POST request: %v", err)
	}
	defer r.Body.Close()
	if _, err := io.Copy(w, r.Body); err != nil {
		return fmt.Errorf("copying response Body: %v", err)
	}
	return nil
}

func post(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", contentType)
	return http.DefaultClient.Do(req.WithContext(ctx))
}

// googleCN reports whether request r is considered
// to be served from golang.google.cn.
func googleCN(r *http.Request) bool {
	if r.FormValue("googlecn") != "" {
		return true
	}
	if strings.HasSuffix(r.Host, ".cn") {
		return true
	}
	if !golangorgenv.CheckCountry() {
		return false
	}
	switch r.Header.Get("X-Appengine-Country") {
	case "", "ZZ", "CN":
		return true
	}
	return false
}
