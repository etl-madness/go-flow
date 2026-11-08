package main

import "testing"

func TestDecodeFileContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
		want string
	}{
		{"utf8", []byte("hello"), "hello"},
		{"utf8-bom", []byte{0xef, 0xbb, 0xbf, 'h', 'i'}, "hi"},
		{"utf16-le", []byte{0xff, 0xfe, 'h', 0, 'i', 0}, "hi"},
		{"utf16-be", []byte{0xfe, 0xff, 0, 'h', 0, 'i'}, "hi"},
		{"truncated-utf16", []byte{0xff, 0xfe, 'x'}, string([]byte{0xff, 0xfe, 'x'})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := decodeFileContent(tc.data); got != tc.want {
				t.Fatalf("decode = %q, want %q", got, tc.want)
			}
		})
	}
}
