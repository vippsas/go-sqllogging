package sqllogging

import (
	"github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
	//"unicode"
)

type scanner struct {
	input string
	pos   int // current position of the scanner
}

func (s *scanner) skipWhitespace() {
	for i, r := range s.input[s.pos:] {
		if !unicode.IsSpace(r) {
			s.pos += i
			return
		}
	}
	// eof
	s.pos = len(s.input)
}

const eof rune = -1

func (s *scanner) peek(offset int) rune {
	if s.pos+offset >= len(s.input) {
		return eof
	} else {
		r, _ := utf8.DecodeRuneInString(s.input[s.pos+offset:])
		return r
	}
}

func parseStringValue(s *scanner) (value string, found bool) {
	// We are positioned after the [.
	// Read until ], while ]] is treated as an escape and produces ]
	// If no trailing single ] is found then found=false is returned

	skipnext := false
	for i, r := range s.input[s.pos:] {
		if skipnext {
			skipnext = false
			continue
		}
		if r == ']' {
			if s.peek(i+1) == ']' {
				skipnext = true
			} else {
				value = strings.ReplaceAll(s.input[s.pos:s.pos+i], "]]", "]")
				s.pos += i + 1
				found = true
				return
			}
		}
	}
	return "", false
}

func parseIntValue(s *scanner) (value int, found bool) {
	// problem here is finding the end of the number, otherwise we could just use Atoi
	newpos := len(s.input) // eof case

loop:
	for i, r := range s.input[s.pos:] {
		switch {
		case i == 0 && (r == '+' || r == '-'):
		case r >= '0' && r <= '9':
		case unicode.IsSpace(r):
			newpos = s.pos + i
			break loop
		default:
			// encountered something not part of numeric and not whitespace => not found
			return 0, false
		}
	}
	value, err := strconv.Atoi(s.input[s.pos:newpos])
	if err != nil {
		// don't see how this could happen, but fall back safely..
		return 0, false
	}
	s.pos = newpos
	found = true
	return
}

type keyValue struct {
	key   string
	value interface{}
}

// parses a key; it is only a key if followed by `=[` or `=\d`,
// otherwise return empty string. The position will be after `=[`
// for stringValue and after `=` for intValue
func parseKeyValue(s *scanner) (kv keyValue, found bool) {
	var key strings.Builder
	var value interface{}
	var valueFound bool

	oldPos := s.pos
loop:
	for i, r := range s.input[s.pos:] {
		if unicode.IsLetter(r) {
			key.WriteRune(r)
		} else if i > 0 && r == '=' {
			s.pos += i + 1
			nextRune := s.peek(0)
			if nextRune == '[' { // string
				s.pos++
				value, valueFound = parseStringValue(s)
				break loop
			} else if unicode.IsSpace(nextRune) || nextRune == eof { // nil
				value = nil
				valueFound = true
				break loop
			} else { // try for integer
				value, valueFound = parseIntValue(s)
				break loop
			}
		} else {
			valueFound = false
			break loop
		}
	}

	if !valueFound {
		s.pos = oldPos
		found = false
		return
	} else {
		kv.key = key.String()
		kv.value = value
		found = true
		return
	}
}

func parseFields(input string) (fields logrus.Fields, msg string) {
	// Top level loop, looking for "key=[value]", skipping but not requiring
	// whitespace in-between

	s := scanner{input: input}

	for {
		s.skipWhitespace()
		kv, found := parseKeyValue(&s)
		if found {
			if fields == nil {
				fields = make(logrus.Fields)
			}
			value := kv.value
			fields[kv.key] = value
		} else {
			msg = s.input[s.pos:]
			return
		}
	}
}
