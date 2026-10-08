package geo

import "testing"

func TestCountryName(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{"known code lower-case", "de", "Germany"},
		{"known code upper-case", "US", "United States"},
		{"uk alias", "uk", "United Kingdom"},
		{"unknown code falls back to upper-case", "zz", "ZZ"},
		{"empty code", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CountryName(tt.code); got != tt.want {
				t.Errorf("CountryName(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestFlagEmoji(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{"germany", "de", "🇩🇪"},
		{"united states", "us", "🇺🇸"},
		{"uk maps to gb", "uk", "🇬🇧"},
		{"gb directly", "gb", "🇬🇧"},
		{"upper-case input", "FR", "🇫🇷"},
		{"unknown two letters still renders", "zz", "\U0001F1FF\U0001F1FF"},
		{"empty falls back", "", "🏳"},
		{"too long falls back", "deu", "🏳"},
		{"digit falls back", "d1", "🏳"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FlagEmoji(tt.code); got != tt.want {
				t.Errorf("FlagEmoji(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}
