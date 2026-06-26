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

// composeJamo turns a Hangul jamo run into precomposed Hangul Syllables.
//
// NFKC is NOT sufficient here. NFKC decomposes Compatibility Jamo to the
// conjoining LEAD position, so a trailing consonant like the ㄴ in 한
// becomes a stray lead jamo: NFKC("ㅎㅏㄴ") → "하ᄂ" instead of "한".
// composeHangulJamo runs a position-aware state machine that picks the
// trailing vs leading interpretation based on the next-character context.
func composeJamo(s string) string {
	if s == "" {
		return s
	}
	// Run the position-aware composer first, then NFC to collapse any
	// remaining conjoining LVT triples (e.g. when the input already
	// contained NFD conjoining jamo).
	return norm.NFC.String(composeHangulJamo(s))
}
