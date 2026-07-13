package main

import "testing"

func TestParseTXT(t *testing.T) {
	tests := []struct {
		name       string
		fields     []string
		wantID     string
		wantCap    string
		wantParsed bool
	}{
		{"well-formed", []string{"v=0", "id=a1b2c3d4", "cap=/v0/capabilities"}, "a1b2c3d4", "/v0/capabilities", true},
		{"missing id", []string{"v=0", "cap=/v0/capabilities"}, "", "/v0/capabilities", false},
		{"missing cap", []string{"v=0", "id=a1b2c3d4"}, "a1b2c3d4", "", false},
		{"empty", nil, "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, capPath, ok := parseTXT(tt.fields)
			if id != tt.wantID || capPath != tt.wantCap || ok != tt.wantParsed {
				t.Errorf("parseTXT(%v) = (%q, %q, %v), want (%q, %q, %v)",
					tt.fields, id, capPath, ok, tt.wantID, tt.wantCap, tt.wantParsed)
			}
		})
	}
}
