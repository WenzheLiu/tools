// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/present"
)

func init() {
	http.HandleFunc("/", dirHandler)
}

func repository(name string) string {
	const repo = "github.com/"
	if !strings.HasPrefix(name, repo) {
		return ""
	}
	name = strings.TrimRight(name, "/")
	cnt := 1
	var i int
	for i = len(repo); i < len(name); i++ {
		if name[i] == '/' {
			cnt++
			if cnt == 3 {
				break
			}
		}
	}
	if cnt < 2 {
		return ""
	}
	return name[:i]
}

func userRepository(name string) (repoPath, dir string) {
	const repo = "users/"
	if !strings.HasPrefix(name, repo) {
		return
	}
	name = strings.TrimRight(name, "/")
	var user, repoName string
	cnt := 1
	var i, last int
	for i = len(repo); i < len(name); i++ {
		if name[i] == '/' {
			switch cnt {
			case 1:
				user = name[len(repo):i]
			case 3:
				repoName = name[last+1 : i]
			}
			if cnt == 1 {
				user = name[len(repo):i]
			}
			cnt++
			last = i
			if cnt == 4 {
				break
			}
		}
	}
	if repoName == "" {
		return
	}
	repoPath = fmt.Sprintf("https://git-brion-us.asml.com:8443/scm/~%s/%s.git", user, repoName)
	dir = name[:i] + "/browse"
	return
}

func sync(name string) error {
	repo, dir := userRepository(name)
	if repo == "" {
		return nil
	}
	fi, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return gitClone(repo, dir)
		}
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf(name, "exists but not dir")
	}
	return nil
}

func gitClone(repo, dirPath string) error {
	dir, name := path.Split(dirPath)
	log.Println("mkdir -p", dir)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}
	cmd := exec.Command("git", "clone", repo, name)
	cmd.Dir = dir
	log.Println("cd", dir)
	log.Println(cmd.Args)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// dirHandler serves a directory listing for the requested path, rooted at *contentPath.
func dirHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/favicon.ico" {
		http.NotFound(w, r)
		return
	}
	name := filepath.ToSlash(filepath.Join(*contentPath, r.URL.Path))
	if isDoc(name) {
		if err := sync(name); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		bf := bytes.Buffer{}
		err := renderDoc(&bf, name)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		renderAgendas(w, bf.Bytes())
		return
	}
	if isRaw(r.URL.Path) {
		r.URL.Path = r.URL.Path[:len(r.URL.Path)-4]
		name = name[:len(name)-4]
	}
	if isDir, err := dirList(w, name); err != nil {
		addr, _, e := net.SplitHostPort(r.RemoteAddr)
		if e != nil {
			addr = r.RemoteAddr
		}
		log.Printf("request from %s: %s", addr, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if isDir {
		return
	}
	http.FileServer(http.Dir(*contentPath)).ServeHTTP(w, r)
}

func isDoc(path string) bool {
	_, ok := contentTemplate[filepath.Ext(path)]
	return ok
}

func isRaw(path string) bool {
	return strings.HasSuffix(path, ".raw")
}

var (
	// dirListTemplate holds the front page template.
	dirListTemplate *template.Template

	// contentTemplate maps the presentable file extensions to the
	// template to be executed.
	contentTemplate map[string]*template.Template
)

func initTemplates(base string) error {
	// Locate the template file.
	actionTmpl := filepath.Join(base, "templates/action.tmpl")

	contentTemplate = make(map[string]*template.Template)

	for ext, contentTmpl := range map[string]string{
		".slide":   "slides.tmpl",
		".article": "article.tmpl",
	} {
		contentTmpl = filepath.Join(base, "templates", contentTmpl)

		// Read and parse the input.
		tmpl := present.Template()
		tmpl = tmpl.Funcs(template.FuncMap{
			"playable": playable,
			"genUML":   genUML,
		})
		if _, err := tmpl.ParseFiles(actionTmpl, contentTmpl); err != nil {
			return err
		}
		contentTemplate[ext] = tmpl
	}

	var err error
	dirListTemplate, err = template.ParseFiles(filepath.Join(base, "templates/dir.tmpl"))
	return err
}

// renderDoc reads the present file, gets its template representation,
// and executes the template, sending output to w.
func renderDoc(w io.Writer, docFile string) error {
	// Read the input and build the doc structure.
	doc, err := parse(docFile, 0)
	if err != nil {
		return err
	}

	// Find which template should be executed.
	tmpl := contentTemplate[filepath.Ext(docFile)]

	// Execute the template.
	return doc.Render(w, tmpl)
}

func parse(name string, mode present.ParseMode) (*present.Doc, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return present.Parse(f, name, mode)
}

// dirList scans the given path and writes a directory listing to w.
// It parses the first part of each .slide file it encounters to display the
// presentation title in the listing.
// If the given path is not a directory, it returns (isDir == false, err == nil)
// and writes nothing to w.
func dirList(w io.Writer, name string) (isDir bool, err error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return false, err
	}
	if isDir = fi.IsDir(); !isDir {
		return false, nil
	}
	fis, err := f.Readdir(0)
	if err != nil {
		return false, err
	}
	strippedPath := strings.TrimPrefix(name, filepath.Clean(*contentPath))
	strippedPath = strings.TrimPrefix(strippedPath, "/")
	d := &dirListData{Path: strippedPath, UrlPrefix: *urlPrefix}
	for _, fi := range fis {
		// skip the golang.org directory
		if name == "." && fi.Name() == "golang.org" {
			continue
		}
		prefix := *urlPrefix
		for prefix != "" && prefix[0] == '/' {
			prefix = prefix[1:]
		}
		e := dirEntry{
			Name: fi.Name(),
			Path: filepath.ToSlash(filepath.Join(prefix, strippedPath, fi.Name())),
		}
		if fi.IsDir() && showDir(e.Name) {
			d.Dirs = append(d.Dirs, e)
			continue
		}
		if isDoc(e.Name) {
			fn := filepath.ToSlash(filepath.Join(name, fi.Name()))
			if p, err := parse(fn, present.TitlesOnly); err != nil {
				log.Printf("parse(%q, present.TitlesOnly): %v", fn, err)
			} else {
				e.Title = p.Title
			}
			switch filepath.Ext(e.Path) {
			case ".article":
				d.Articles = append(d.Articles, e)
			case ".slide":
				d.Slides = append(d.Slides, e)
			}
		} else if showFile(e.Name) {
			d.Other = append(d.Other, e)
		}
	}
	if d.Path == "." {
		d.Path = ""
	}
	sort.Sort(d.Dirs)
	sort.Sort(d.Slides)
	sort.Sort(d.Articles)
	sort.Sort(d.Other)
	return true, dirListTemplate.Execute(w, d)
}

// showFile reports whether the given file should be displayed in the list.
func showFile(n string) bool {
	switch filepath.Ext(n) {
	case ".pdf":
	case ".html":
	case ".go":
	default:
		return isDoc(n)
	}
	return true
}

// showDir reports whether the given directory should be displayed in the list.
func showDir(n string) bool {
	if len(n) > 0 && (n[0] == '.' || n[0] == '_') || n == "present" {
		return false
	}
	return true
}

type dirListData struct {
	Path                          string
	Dirs, Slides, Articles, Other dirEntrySlice
	UrlPrefix                     string
}

type dirEntry struct {
	Name, Path, Title string
}

type dirEntrySlice []dirEntry

func (s dirEntrySlice) Len() int           { return len(s) }
func (s dirEntrySlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s dirEntrySlice) Less(i, j int) bool { return s[i].Name < s[j].Name }
