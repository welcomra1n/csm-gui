package main

// Hangul jamo composer.
//
// We have to do this ourselves because:
//
//   - NFKC decomposes Hangul Compatibility Jamo (U+3131..U+318E) to the
//     CONJOINING LEAD jamo position. So NFKC("ㅎㅏㄴ") gives "하ᄂ"
//     (LV + a stray lead jamo) instead of "한" (LVT). Compat jamo carry
//     no position information; NFKC has no way to know the user meant
//     ㄴ to be the trailing consonant of 한 instead of the lead of a
//     new syllable.
//
//   - macOS WKWebView occasionally hands the textarea raw compat jamo
//     for each keystroke instead of going through the IME's composed-
//     syllable path. Once we receive that stream we have to compose it
//     ourselves or the TUI ends up rendering ㅎ-ㅏ-ㄴ-ㄱ-ㅡ-ㄹ-ㅗ
//     instead of 한글로.
//
// Algorithm: scan runes left to right keeping a single in-progress
// syllable (pendL, pendV, pendT). For each new jamo decide whether it
// belongs to the current syllable or starts a fresh one. The only
// genuinely ambiguous case is a single L-capable consonant arriving
// after an LV pair (e.g. "ㅎㅏ" + "ㄴ"): the consonant is the trailing
// consonant of the current syllable iff it is NOT immediately followed
// by a vowel (in which case it would be the lead of the next syllable).
// One-character lookahead resolves it.
//
// Compound trail consonants (ㄳ ㄵ ㄶ ㄺ ㄻ ㄼ ㄽ ㄾ ㄿ ㅀ ㅄ) are
// formed when two T-capable consonants land back-to-back in trailing
// position with no following vowel — this matches how the system 2-set
// keyboard behaves.

type jamoKind int

const (
	jNone        jamoKind = iota
	jLeadOnly             // L only (ㄸ ㅃ ㅉ — never trailing)
	jLeadOrTrail          // most consonants can be either
	jTrailOnly            // already-compound trail consonants
	jVowel
)

type jamoEntry struct {
	kind  jamoKind
	lead  rune // conjoining L (U+1100..U+1112), 0 if none
	vowel rune // conjoining V (U+1161..U+1175), 0 if none
	trail rune // conjoining T (U+11A8..U+11C2), 0 if none
}

// compatJamoTable maps Hangul Compatibility Jamo to their conjoining
// equivalents plus a hint about which positions they can occupy.
// Indices follow U+3131..U+3163 contiguously.
var compatJamoTable = map[rune]jamoEntry{
	0x3131: {jLeadOrTrail, 0x1100, 0, 0x11A8}, // ㄱ
	0x3132: {jLeadOrTrail, 0x1101, 0, 0x11A9}, // ㄲ
	0x3133: {jTrailOnly, 0, 0, 0x11AA},        // ㄳ
	0x3134: {jLeadOrTrail, 0x1102, 0, 0x11AB}, // ㄴ
	0x3135: {jTrailOnly, 0, 0, 0x11AC},        // ㄵ
	0x3136: {jTrailOnly, 0, 0, 0x11AD},        // ㄶ
	0x3137: {jLeadOrTrail, 0x1103, 0, 0x11AE}, // ㄷ
	0x3138: {jLeadOnly, 0x1104, 0, 0},         // ㄸ
	0x3139: {jLeadOrTrail, 0x1105, 0, 0x11AF}, // ㄹ
	0x313A: {jTrailOnly, 0, 0, 0x11B0},        // ㄺ
	0x313B: {jTrailOnly, 0, 0, 0x11B1},        // ㄻ
	0x313C: {jTrailOnly, 0, 0, 0x11B2},        // ㄼ
	0x313D: {jTrailOnly, 0, 0, 0x11B3},        // ㄽ
	0x313E: {jTrailOnly, 0, 0, 0x11B4},        // ㄾ
	0x313F: {jTrailOnly, 0, 0, 0x11B5},        // ㄿ
	0x3140: {jTrailOnly, 0, 0, 0x11B6},        // ㅀ
	0x3141: {jLeadOrTrail, 0x1106, 0, 0x11B7}, // ㅁ
	0x3142: {jLeadOrTrail, 0x1107, 0, 0x11B8}, // ㅂ
	0x3143: {jLeadOnly, 0x1108, 0, 0},         // ㅃ
	0x3144: {jTrailOnly, 0, 0, 0x11B9},        // ㅄ
	0x3145: {jLeadOrTrail, 0x1109, 0, 0x11BA}, // ㅅ
	0x3146: {jLeadOrTrail, 0x110A, 0, 0x11BB}, // ㅆ
	0x3147: {jLeadOrTrail, 0x110B, 0, 0x11BC}, // ㅇ
	0x3148: {jLeadOrTrail, 0x110C, 0, 0x11BD}, // ㅈ
	0x3149: {jLeadOnly, 0x110D, 0, 0},         // ㅉ
	0x314A: {jLeadOrTrail, 0x110E, 0, 0x11BE}, // ㅊ
	0x314B: {jLeadOrTrail, 0x110F, 0, 0x11BF}, // ㅋ
	0x314C: {jLeadOrTrail, 0x1110, 0, 0x11C0}, // ㅌ
	0x314D: {jLeadOrTrail, 0x1111, 0, 0x11C1}, // ㅍ
	0x314E: {jLeadOrTrail, 0x1112, 0, 0x11C2}, // ㅎ
	0x314F: {jVowel, 0, 0x1161, 0},            // ㅏ
	0x3150: {jVowel, 0, 0x1162, 0},            // ㅐ
	0x3151: {jVowel, 0, 0x1163, 0},            // ㅑ
	0x3152: {jVowel, 0, 0x1164, 0},            // ㅒ
	0x3153: {jVowel, 0, 0x1165, 0},            // ㅓ
	0x3154: {jVowel, 0, 0x1166, 0},            // ㅔ
	0x3155: {jVowel, 0, 0x1167, 0},            // ㅕ
	0x3156: {jVowel, 0, 0x1168, 0},            // ㅖ
	0x3157: {jVowel, 0, 0x1169, 0},            // ㅗ
	0x3158: {jVowel, 0, 0x116A, 0},            // ㅘ
	0x3159: {jVowel, 0, 0x116B, 0},            // ㅙ
	0x315A: {jVowel, 0, 0x116C, 0},            // ㅚ
	0x315B: {jVowel, 0, 0x116D, 0},            // ㅛ
	0x315C: {jVowel, 0, 0x116E, 0},            // ㅜ
	0x315D: {jVowel, 0, 0x116F, 0},            // ㅝ
	0x315E: {jVowel, 0, 0x1170, 0},            // ㅞ
	0x315F: {jVowel, 0, 0x1171, 0},            // ㅟ
	0x3160: {jVowel, 0, 0x1172, 0},            // ㅠ
	0x3161: {jVowel, 0, 0x1173, 0},            // ㅡ
	0x3162: {jVowel, 0, 0x1174, 0},            // ㅢ
	0x3163: {jVowel, 0, 0x1175, 0},            // ㅣ
}

// jamoLookup returns the composition info for a rune. Conjoining jamo
// (U+1100..U+11FF range) are returned as already-positioned entries so
// the composer can mix the two encodings in one pass.
func jamoLookup(r rune) jamoEntry {
	if e, ok := compatJamoTable[r]; ok {
		return e
	}
	switch {
	case r >= 0x1100 && r <= 0x1112:
		// Conjoining L
		return jamoEntry{jLeadOnly, r, 0, 0}
	case r >= 0x1161 && r <= 0x1175:
		// Conjoining V
		return jamoEntry{jVowel, 0, r, 0}
	case r >= 0x11A8 && r <= 0x11C2:
		// Conjoining T
		return jamoEntry{jTrailOnly, 0, 0, r}
	}
	return jamoEntry{jNone, 0, 0, 0}
}

// trailCompoundTable maps (existing trail, incoming trail-capable
// consonant) → combined trail. Mirrors how the 2-set keyboard collapses
// e.g. ㄴ + ㅎ into ㄶ when both land in the trailing position.
var trailCompoundTable = map[[2]rune]rune{
	{0x11A8, 0x11BA}: 0x11AA, // ㄱ + ㅅ → ㄳ
	{0x11AB, 0x11BD}: 0x11AC, // ㄴ + ㅈ → ㄵ
	{0x11AB, 0x11C2}: 0x11AD, // ㄴ + ㅎ → ㄶ
	{0x11AF, 0x11A8}: 0x11B0, // ㄹ + ㄱ → ㄺ
	{0x11AF, 0x11B7}: 0x11B1, // ㄹ + ㅁ → ㄻ
	{0x11AF, 0x11B8}: 0x11B2, // ㄹ + ㅂ → ㄼ
	{0x11AF, 0x11BA}: 0x11B3, // ㄹ + ㅅ → ㄽ
	{0x11AF, 0x11C0}: 0x11B4, // ㄹ + ㅌ → ㄾ
	{0x11AF, 0x11C1}: 0x11B5, // ㄹ + ㅍ → ㄿ
	{0x11AF, 0x11C2}: 0x11B6, // ㄹ + ㅎ → ㅀ
	{0x11B8, 0x11BA}: 0x11B9, // ㅂ + ㅅ → ㅄ
}

// trailToLead converts a conjoining trail jamo to its lead counterpart
// when possible. Used for the "borrow back" case: typing "한" + "아"
// reinterprets the trailing ㄴ as the lead of the new syllable, leaving
// "하" + "나".
var trailToLeadTable = map[rune]rune{
	0x11A8: 0x1100, 0x11A9: 0x1101,
	0x11AB: 0x1102,
	0x11AE: 0x1103,
	0x11AF: 0x1105,
	0x11B7: 0x1106,
	0x11B8: 0x1107,
	0x11BA: 0x1109, 0x11BB: 0x110A,
	0x11BC: 0x110B,
	0x11BD: 0x110C,
	0x11BE: 0x110E,
	0x11BF: 0x110F,
	0x11C0: 0x1110,
	0x11C1: 0x1111,
	0x11C2: 0x1112,
}

// composeHangulJamo turns a string of Hangul jamo (compat OR conjoining,
// freely mixed) into properly composed Hangul Syllables wherever the
// L+V[+T] pattern allows it. Non-jamo runes pass through untouched.
func composeHangulJamo(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	var out []rune
	var L, V, T rune // 0 when slot empty; otherwise the conjoining form

	emit := func() {
		if L == 0 && V == 0 && T == 0 {
			return
		}
		if L != 0 && V != 0 {
			// Precomposed syllable formula: (L-0x1100)*588 + (V-0x1161)*28 + (T?T-0x11A7:0) + 0xAC00
			Li := int(L - 0x1100)
			Vi := int(V - 0x1161)
			Ti := 0
			if T != 0 {
				Ti = int(T - 0x11A7)
			}
			out = append(out, rune(Li*588+Vi*28+Ti+0xAC00))
		} else {
			// Incomplete syllable — emit whichever jamo we have, as compat
			// so they render in the wide standalone glyph rather than the
			// half-width conjoining form.
			if L != 0 {
				out = append(out, leadToCompat(L))
			}
			if V != 0 {
				out = append(out, vowelToCompat(V))
			}
			if T != 0 {
				out = append(out, trailToCompat(T))
			}
		}
		L, V, T = 0, 0, 0
	}

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		info := jamoLookup(r)
		if info.kind == jNone {
			// Non-jamo — flush current syllable then pass through.
			emit()
			out = append(out, r)
			continue
		}

		switch info.kind {
		case jVowel:
			if L == 0 {
				// Vowel with no lead — flush and emit standalone.
				emit()
				out = append(out, r)
				continue
			}
			if V == 0 {
				V = info.vowel
				continue
			}
			// Already had V (and maybe T). New V starts a fresh syllable.
			// If we hold a trailing consonant, it actually belongs to the
			// new syllable as its lead — pop it back.
			if T != 0 {
				if newLead, ok := trailToLeadTable[T]; ok {
					popped := newLead
					T = 0
					emit() // emit the old LV
					L = popped
					V = info.vowel
				} else {
					emit()
					out = append(out, r)
				}
			} else {
				emit()
				out = append(out, r)
			}

		case jLeadOnly:
			// Consonant that can only be a lead (e.g. ㄸ).
			emit()
			L = info.lead

		case jLeadOrTrail:
			if L == 0 {
				// Start new syllable with this consonant as lead.
				L = info.lead
				continue
			}
			if V == 0 {
				// Two leads back to back — flush the first as a standalone
				// consonant and start a new syllable.
				emit()
				L = info.lead
				continue
			}
			// State is LV or LVT. Decide: trailing of current, or lead of next?
			// Trailing requires (a) a trail form exists and (b) the next
			// rune is NOT a vowel (else this consonant is the next L).
			nextIsVowel := false
			if i+1 < len(runes) {
				ni := jamoLookup(runes[i+1])
				if ni.kind == jVowel {
					nextIsVowel = true
				}
			}
			if !nextIsVowel && info.trail != 0 {
				if T == 0 {
					T = info.trail
					continue
				}
				// LVT already — try compound trail
				if combined, ok := trailCompoundTable[[2]rune{T, info.trail}]; ok {
					T = combined
					continue
				}
				emit()
				L = info.lead
				continue
			}
			// Next is vowel (or no trail form) — this consonant is the next L.
			emit()
			L = info.lead

		case jTrailOnly:
			// Compound trailing consonant (ㄳ etc.) — only fits in T slot.
			if V != 0 && T == 0 {
				T = info.trail
				continue
			}
			emit()
			out = append(out, r)
		}
	}
	emit()
	return string(out)
}

// leadToCompat / vowelToCompat / trailToCompat fall back to the compat
// glyph when we have to emit an incomplete syllable. The compat block
// is what every common monospace font renders as a clean wide jamo;
// raw conjoining jamo render half-width in most terminals.
func leadToCompat(r rune) rune {
	// L range U+1100..U+1112 maps roughly into compat block; use lookup.
	for c, e := range compatJamoTable {
		if e.lead == r {
			return c
		}
	}
	return r
}
func vowelToCompat(r rune) rune {
	for c, e := range compatJamoTable {
		if e.vowel == r {
			return c
		}
	}
	return r
}
func trailToCompat(r rune) rune {
	for c, e := range compatJamoTable {
		if e.trail == r {
			return c
		}
	}
	return r
}
