// Package geo provides country display names and flag emoji helpers
// for the country selection menu.
//
// This package is a leaf: it must not import any other internal
// package.
package geo

import "strings"

// names maps lower-case country codes (including the uk alias used
// by Speedify) to display names.
var names = map[string]string{
	"ae": "United Arab Emirates", "al": "Albania", "am": "Armenia", "ar": "Argentina", "at": "Austria",
	"au": "Australia", "az": "Azerbaijan", "ba": "Bosnia & Herzegovina", "bd": "Bangladesh", "be": "Belgium",
	"bg": "Bulgaria", "br": "Brazil", "by": "Belarus", "ca": "Canada", "ch": "Switzerland", "cl": "Chile",
	"co": "Colombia", "cr": "Costa Rica", "cy": "Cyprus", "cz": "Czechia", "de": "Germany", "dk": "Denmark",
	"ec": "Ecuador", "ee": "Estonia", "eg": "Egypt", "es": "Spain", "fi": "Finland", "fr": "France",
	"gb": "United Kingdom", "uk": "United Kingdom", "ge": "Georgia", "gr": "Greece", "hk": "Hong Kong",
	"hr": "Croatia", "hu": "Hungary", "id": "Indonesia", "ie": "Ireland", "il": "Israel", "in": "India",
	"is": "Iceland", "it": "Italy", "jp": "Japan", "ke": "Kenya", "kr": "South Korea", "kz": "Kazakhstan",
	"lk": "Sri Lanka", "lt": "Lithuania", "lu": "Luxembourg", "lv": "Latvia", "md": "Moldova",
	"me": "Montenegro", "mk": "North Macedonia", "mt": "Malta", "mx": "Mexico", "my": "Malaysia",
	"ng": "Nigeria", "nl": "Netherlands", "no": "Norway", "nz": "New Zealand", "pa": "Panama", "pe": "Peru",
	"ph": "Philippines", "pk": "Pakistan", "pl": "Poland", "pt": "Portugal", "ro": "Romania", "rs": "Serbia",
	"ru": "Russia", "sa": "Saudi Arabia", "se": "Sweden", "sg": "Singapore", "si": "Slovenia",
	"sk": "Slovakia", "th": "Thailand", "tr": "Turkey", "tw": "Taiwan", "ua": "Ukraine",
	"us": "United States", "uy": "Uruguay", "vn": "Vietnam", "za": "South Africa",
}

// CountryName returns the display name for a country code in any case,
// falling back to the upper-cased code when the code is unknown.
func CountryName(code string) string {
	if n, ok := names[strings.ToLower(code)]; ok {
		return n
	}
	return strings.ToUpper(code)
}

// FlagEmoji returns the regional-indicator flag emoji for a two-letter
// country code. The uk alias maps to gb; malformed codes return a
// white flag.
func FlagEmoji(code string) string {
	code = strings.ToUpper(code)
	if code == "UK" {
		code = "GB"
	}
	r := []rune(code)
	if len(r) != 2 || r[0] < 'A' || r[0] > 'Z' || r[1] < 'A' || r[1] > 'Z' {
		return "🏳"
	}
	return string([]rune{0x1F1E6 + (r[0] - 'A'), 0x1F1E6 + (r[1] - 'A')})
}
