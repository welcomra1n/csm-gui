package main

import "golang.org/x/text/unicode/norm"

// nfc normalises a UTF-8 string to Unicode NFC. macOS filesystem APIs return
// paths in NFD, which causes Korean Hangul syllables to render as detached
// jamo (e.g. "한글" → "ㅎㅏㄴㄱㅡㄹ") in the webview. Run any filesystem-derived
// string through this before exposing it to the frontend.
func nfc(s string) string {
	if s == "" {
		return s
	}
	return norm.NFC.String(s)
}
