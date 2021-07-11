// Package hidden provides the ability to "hide" binary data in a string using
// a hex encoding with non-printing characters. Hidden data is demarcated with
// a leading and trailing NUL character.
package hidden

import (
	"bytes"
	"fmt"
	"regexp"

	"github.com/getlantern/hex"
)

const (
	// 16 non-printing characters
	hextable = "\x01\x02\x03\x04\x05\x06\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17"

	null = '\x00'
	hex1 = '\x06'
	hex2 = '\x0e'
	hex3 = '\x17'
)

var (
	hexencoding = hex.NewEncoding(hextable)

	re *regexp.Regexp
)

func init() {
	var err error
	re, err = regexp.Compile(fmt.Sprintf("\x00[%v]+\x00", hextable))
	if err != nil {
		panic(err)
	}
}

// ToString encodes the given data as a hidden string, including leadnig and
// trailing NULs.
func ToString(data []byte) string {
	buf := bytes.NewBuffer(make([]byte, 0, 2+hex.EncodedLen(len(data))))
	// Leading NUL
	buf.WriteByte(0)
	buf.WriteString(hexencoding.EncodeToString(data))
	// Trailing NUL
	buf.WriteByte(0)
	return buf.String()
}

// FromString extracts the hidden data from a string, which is expected to
// contain leading and trailing NULs.
func FromString(str string) ([]byte, error) {
	return hexencoding.DecodeString(str[1 : len(str)-1])
}

// Extract extracts all hidden data from an arbitrary string.
func Extract(str string) ([][]byte, error) {
	m := re.FindAllString(str, -1)
	result := make([][]byte, 0, len(m))
	for _, s := range m {
		b, err := FromString(s)
		if err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, nil
}

// Clean removes all control characters that might be encoding hidden data from a given string.
func Clean(str string) string {
	result := make([]byte, 0, len(str))
	for _, b := range []byte(str) {
		isNotHidden := b > hex3 || (b > hex1 && b < hex2)
		if isNotHidden {
			result = append(result, b)
		}
	}
	return string(result)
}
