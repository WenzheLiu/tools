package main

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestRenderAgendas(t *testing.T)  {
	fname := "/tmp/ppt.html"
	bs, err := ioutil.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.OpenFile("/tmp/ppt_out.html", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	renderAgendas(f, bs)
}

