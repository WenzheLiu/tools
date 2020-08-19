// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package present

import (
	"bytes"
	"html"
	"html/template"
	"strings"
	"unicode"
	"unicode/utf8"
)

/*
	Fonts are demarcated by an initial and final char bracketing a
	space-delimited word, plus possibly some terminal punctuation.
	The chars are
		_ for italic
		* for bold
		` (back quote) for fixed width.
	Inner appearances of the char become spaces. For instance,
		_this_is_italic_!
	becomes
		<i>this is italic</i>!
*/

func init() {
	funcs["style"] = Style
}

// Style returns s with HTML entities escaped and font indicators turned into
// HTML font tags.
func Style(s string) template.HTML {
	return template.HTML(font(html.EscapeString(s)))
}

// font returns s with font indicators turned into HTML font tags.
func font(s string) string {
	if !strings.ContainsAny(s, "[`_*") {
		return s
	}

	skipSplit := false
	if len(s) > 2 {
		for _, tag := range []byte("`_*") {
			if s[0] == tag && s[len(s)-1] == tag {
				skipSplit = true
				break
			}
		}
	}

	var words []string
	if skipSplit {
		words = []string{s}
	} else {
		words = split(s)
	}
	var b bytes.Buffer
Word:
	for w, word := range words {
		if len(word) < 2 {
			continue Word
		}
		if link, _ := parseInlineLink(word); link != "" {
			words[w] = link
			continue Word
		}
		const marker = "_*`"
		// Initial punctuation is OK but must be peeled off.
		first := strings.IndexAny(word, marker)
		if first == -1 {
			continue Word
		}
		// Opening marker must be at the beginning of the token or else preceded by punctuation.
		if first != 0 {
			r, _ := utf8.DecodeLastRuneInString(word[:first])
			if !unicode.IsPunct(r) {
				continue Word
			}
		}
		open, word := word[:first], word[first:]
		char := word[0] // ASCII is OK.
		close := ""

		const quote = "&#34;"
		const style = "style=" + quote
		css := ""
		if len(word) > 1 + len(style) + len(quote) && strings.HasPrefix(word[1:], style) {
			q := strings.Index(word[1 + len(style):], quote)
			if q != -1 {
				css = word[1+len(style):1+len(style)+q]
			}
		}

		switch char {
		default:
			continue Word
		case '_':
			if css == "" {
				open += "<i>"
			} else {
				open += "<i style=" + css + ">"
			}
			close = "</i>"
		case '*':
			if css == "" {
				open += "<b>"
			} else {
				open += "<b style=" + css + ">"
			}
			close = "</b>"
		case '`':
			if css == "" {
				open += "<code>"
			} else {
				open += "<code style=" + css + ">"
			}
			close = "</code>"
		}
		// Closing marker must be at the end of the token or else followed by punctuation.
		last := strings.LastIndex(word, word[:1])
		if last == 0 {
			continue Word
		}
		if last+1 != len(word) {
			r, _ := utf8.DecodeRuneInString(word[last+1:])
			if !unicode.IsPunct(r) {
				continue Word
			}
		}
		head, tail := word[:last+1], word[last+1:]
		b.Reset()
		b.WriteString(open)
		var wid int
		start := 1
		if css != "" {
			start += len(style) + len(css) + len(quote)
		}
		for i := start; i < len(head)-1; i += wid {
			var r rune
			r, wid = utf8.DecodeRuneInString(head[i:])
			if r != rune(char) {
				// Ordinary character.
				b.WriteRune(r)
				continue
			}
			if head[i+1] != char {
				// Inner char becomes space.
				b.WriteRune(' ')
				continue
			}
			// Doubled char becomes real char.
			// Not worth worrying about "_x__".
			b.WriteByte(char)
			wid++ // Consumed two chars, both ASCII.
		}
		b.WriteString(close) // Write closing tag.
		b.WriteString(tail)  // Restore trailing punctuation.
		words[w] = b.String()
	}
	return strings.Join(words, "")
}

// split is like strings.Fields but also returns the runs of spaces
// and treats inline links as distinct words.
func split(s string) []string {
	var (
		words = make([]string, 0, 10)
		start = 0
	)

	// appendWord appends the string s[start:end] to the words slice.
	// If the word contains the beginning of a link, the non-link portion
	// of the word and the entire link are appended as separate words,
	// and the start index is advanced to the end of the link.
	appendWord := func(end int) {
		if j := strings.Index(s[start:end], "[["); j > -1 {
			if _, l := parseInlineLink(s[start+j:]); l > 0 {
				// Append portion before link, if any.
				if j > 0 {
					words = append(words, s[start:start+j])
				}
				// Append link itself.
				words = append(words, s[start+j:start+j+l])
				// Advance start index to end of link.
				start = start + j + l
				return
			}
		}
		// No link; just add the word.
		words = append(words, s[start:end])
		start = end
	}

	wasSpace := false
	for i, r := range s {
		isSpace := unicode.IsSpace(r)
		if i > start && isSpace != wasSpace {
			appendWord(i)
		}
		wasSpace = isSpace
	}
	for start < len(s) {
		appendWord(len(s))
	}
	return words
}
