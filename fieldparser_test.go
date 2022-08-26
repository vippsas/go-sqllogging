package sqllog

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestParseIntValue(t *testing.T) {
	tests := []struct {
		input  string
		number int
		found  bool
	}{
		{input: "+123", number: 123, found: true},
		{input: "+123  asdf", number: 123, found: true},
		{input: "-1", number: -1, found: true},
		{input: "123  asdf", number: 123, found: true},
		{input: "123asdf", number: 0, found: false},
		{input: " 123", number: 0, found: false},
		{input: "x123", number: 0, found: false},
		{input: "", number: 0, found: false},
	}

	for i, tc := range tests {
		testname := fmt.Sprintf("Test %d", i)
		s := scanner{input: "abcd" + tc.input, pos: 4}
		number, found := parseIntValue(&s)
		assert.Equal(t, tc.number, number, testname)
		assert.Equal(t, tc.found, found, testname)
	}
}

func TestParseStringValue(t *testing.T) {
	tests := []struct {
		input, value, remainder string
		found                   bool
	}{
		{input: "hello world] remainder", value: "hello world", remainder: " remainder", found: true},
		{input: "hello world]] part of value]", value: "hello world] part of value", remainder: "", found: true},
		{input: "]]]", value: "]", remainder: "", found: true},
		{input: "]]]]]", value: "]]", remainder: "", found: true},
		{input: "", value: "", remainder: "", found: false},
		{input: "]", value: "", remainder: "", found: true},
		{input: "no bracket at the end", found: false},
	}
	for _, tc := range tests {
		s := scanner{input: "abcd" + tc.input, pos: 4}
		value, found := parseStringValue(&s)
		require.Equal(t, tc.found, found)
		if found {
			require.Equal(t, tc.value, value)
			require.Equal(t, tc.remainder, tc.input[s.pos:])
		}
	}
}

func TestParseKeyValuePair(t *testing.T) {
	tests := []struct {
		input string
		kv    keyValue
		found bool
	}{
		{input: "a=1", kv: keyValue{"a", 1}, found: true},
		{input: "abc=[1]", kv: keyValue{"abc", "1"}, found: true},
		{input: "a=1  ", kv: keyValue{"a", 1}, found: true},
		{input: "abc=[1]  ", kv: keyValue{"abc", "1"}, found: true},
		{input: "abc=", found: false},
		{input: " a=1", found: false},
	}
	for i, tc := range tests {
		testname := fmt.Sprintf("Test %d", i)
		s := scanner{input: "abcd" + tc.input, pos: 4}
		kv, found := parseKeyValue(&s)
		assert.Equal(t, tc.found, found, testname)
		if found {
			assert.Equal(t, tc.kv, kv, testname)
		} else {
			assert.Equal(t, 4, s.pos)
		}
	}

}

func TestParseFields(t *testing.T) {
	tests := []struct {
		input  string
		fields logrus.Fields
		msg    string
	}{
		{
			input:  "one=[aaa]two=[bbb]three=3",
			fields: logrus.Fields{"one": "aaa", "two": "bbb", "three": 3},
			msg:    "",
		},
		{
			input:  "a=[1]b=[2 ]] \" /*  asdf */ lots of junk ]]] msg with [] ]] c=[3]",
			fields: logrus.Fields{"a": "1", "b": "2 ] \" /*  asdf */ lots of junk ]"},
			msg:    "msg with [] ]] c=[3]",
		},
		{
			// space before =; syntax error
			input:  "a =[1]b=[2]c=[3]",
			fields: nil,
			msg:    "a =[1]b=[2]c=[3]",
		},
		{
			// allow whitespace between entries
			input:  "a=[1] \t \n b=[2]  \t \t \t \n c=[3] \t  \n   msg",
			fields: logrus.Fields{"a": "1", "b": "2", "c": "3"},
			msg:    "msg",
		},
	}
	for _, tc := range tests {
		fields, msg := parseFields(tc.input)
		require.Equal(t, tc.fields, fields)
		require.Equal(t, tc.msg, msg)
	}
}
