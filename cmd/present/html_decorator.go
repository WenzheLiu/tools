package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Agenda struct {
	Start int // start index in html (included)
	End int   // end index in html (excluded)
	Content string
	PageNum int
}

func renderAgendas(w io.Writer, bs []byte)  {
	html := string(bs)
	const (
		startTag = "<h2>"
		endTag = "</h2>"
	)
	agendas := make([]Agenda, 0, 8)
	end := 0
	for {
		start := strings.Index(html[end:], startTag)
		if start < 0 {
			break
		}
		start += end

		// check <article > should just before start
		i := start - 1
		for ; i >= 0 && (bs[i] == ' ' || bs[i] == '\n'); i-- {}
		i++
		const articleTag = "<article >"
		if i < len(articleTag) || html[i-len(articleTag):i] != articleTag {
			end = start + len(startTag)
			continue
		}

		r := Agenda{}
		r.Start = start

		start += len(startTag)
		end = strings.Index(html[start:], endTag)
		if end < 0 {
			break
		}
		end += start

		r.Content = html[start:end]

		end += len(endTag)
		r.End = end

		// check <span class="pagenumber">2</span> should just after end
		i = end
		for ; i < len(bs) && (bs[i] == ' ' || bs[i] == '\n'); i++ {}
		const spanTag = `<span class="pagenumber">`
		if i + len(spanTag) >= len(bs) || html[i:i+len(spanTag)] != spanTag {
			continue
		}

		// get page number
		i += len(spanTag)
		j := i
		for ; j < len(bs) && bs[j] >= '0' && bs[j] <= '9'; j++ {}
		if i == j || j == len(bs) {
			continue
		}
		r.PageNum, _ = strconv.Atoi(html[i:j])

		agendas = append(agendas, r)
	}
	end = 0
	for i, r := range agendas {
		w.Write(bs[end:r.Start])
		renderAgenda(w, agendas, i)
		end = r.End
	}
	w.Write(bs[end:])
}

func renderAgenda(w io.Writer, rs []Agenda, index int) {
	w.Write([]byte("<h3>Agenda</h3>\n<ul>\n"))
	for i, r := range rs {
		w.Write([]byte("  <li>"))
		if i == index {
			fmt.Fprintf(w, `<span style="color:deeppink">%s</span>`, r.Content)
		} else {
			fmt.Fprintf(w, `<a href="javascript:void(0);" onclick="openSlide(%d)">%s</a>`, r.PageNum, r.Content)
		}
		w.Write([]byte("</li>\n"))
	}
	w.Write([]byte("</ul>"))
}
