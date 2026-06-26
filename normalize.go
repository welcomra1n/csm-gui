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

// composeJamo aggressively folds a Hangul jamo run into precomposed
// syllables. Use only on strings that contain ONLY Hangul jamo runes
// (validated upstream by isHangulJamo), because NFKC has other effects
// (fullwidth → halfwidth, ligature breaking, etc.) that we do not want
// for general terminal input.
//
// Why NFKC and not NFC: macOS WKWebView sometimes hands us Hangul
// Compatibility Jamo (U+3131..U+318E) on keystrokes — these are the
// "display jamo" in the Hangul keyboard layout, not the conjoining
// jamo. NFC alone cannot combine them. NFKC's compatibility
// decomposition step rewrites them to conjoining jamo (U+1100..U+11FF),
// and the canonical composition step then folds those into precomposed
// syllables in the Hangul Syllables block.
func composeJamo(s string) string {
	if s == "" {
		return s
	}
	return norm.NFKC.String(s)
}
