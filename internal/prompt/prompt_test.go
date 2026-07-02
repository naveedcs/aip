package prompt

import (
	"strings"
	"testing"
)

func TestTextUsesDefaultOnEmpty(t *testing.T) {
	var out strings.Builder
	got, err := New(strings.NewReader("\n"), &out).Text("Name", "default-name")
	if err != nil {
		t.Fatalf("Text returned error: %v", err)
	}
	if got != "default-name" {
		t.Fatalf("Text() = %q, want %q", got, "default-name")
	}
	if !strings.Contains(out.String(), "Name") {
		t.Fatalf("prompt output %q does not contain label", out.String())
	}
}

func TestTextReturnsInput(t *testing.T) {
	var out strings.Builder
	got, err := New(strings.NewReader("  custom value  \r\n"), &out).Text("Name", "default-name")
	if err != nil {
		t.Fatalf("Text returned error: %v", err)
	}
	if got != "custom value" {
		t.Fatalf("Text() = %q, want %q", got, "custom value")
	}
}

func TestPasswordFallsBackToLineReadForNonTerminalInput(t *testing.T) {
	var out strings.Builder
	got, err := New(strings.NewReader("  s3cr3t  \n"), &out).Password("Secret value")
	if err != nil {
		t.Fatalf("Password returned error: %v", err)
	}
	if got != "  s3cr3t  " {
		t.Fatalf("Password() = %q, want %q", got, "  s3cr3t  ")
	}
	if !strings.Contains(out.String(), "Secret value: ") {
		t.Fatalf("prompt output %q does not contain password label", out.String())
	}
}

func TestConfirmParsesAnswers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		def   bool
		want  bool
	}{
		{name: "y true", input: "y\n", want: true},
		{name: "yes true", input: "YES\n", want: true},
		{name: "n false", input: "n\n", want: false},
		{name: "no false", input: "No\n", want: false},
		{name: "empty default true", input: "\n", def: true, want: true},
		{name: "empty default false", input: "\n", def: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			got, err := New(strings.NewReader(tt.input), &out).Confirm("Continue", tt.def)
			if err != nil {
				t.Fatalf("Confirm returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Confirm() = %v, want %v", got, tt.want)
			}
		})
	}

	var out strings.Builder
	_, err := New(strings.NewReader("maybe\n"), &out).Confirm("Continue", false)
	if err == nil {
		t.Fatal("Confirm accepted invalid input")
	}
}

func TestSelectReturnsIndex(t *testing.T) {
	var out strings.Builder
	got, err := New(strings.NewReader("2\n"), &out).Select("Pick one", testOptions(), 0)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}
	if got != 1 {
		t.Fatalf("Select() = %d, want %d", got, 1)
	}
	if !strings.Contains(out.String(), "1. Alpha") || !strings.Contains(out.String(), "2. Beta") {
		t.Fatalf("menu output %q does not contain numbered options", out.String())
	}
}

func TestSelectDefaultOnEmpty(t *testing.T) {
	var out strings.Builder
	got, err := New(strings.NewReader("\n"), &out).Select("Pick one", testOptions(), 2)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}
	if got != 2 {
		t.Fatalf("Select() = %d, want %d", got, 2)
	}
}

func TestSelectRejectsOutOfRange(t *testing.T) {
	var out strings.Builder
	_, err := New(strings.NewReader("4\n"), &out).Select("Pick one", testOptions(), 0)
	if err == nil {
		t.Fatal("Select accepted out-of-range input")
	}
}

func TestMultiSelectParsesList(t *testing.T) {
	var out strings.Builder
	got, err := New(strings.NewReader("3, 1\n"), &out).MultiSelect("Pick many", testOptions(), nil)
	if err != nil {
		t.Fatalf("MultiSelect returned error: %v", err)
	}
	want := []int{2, 0}
	if !sameInts(got, want) {
		t.Fatalf("MultiSelect() = %v, want %v", got, want)
	}
}

func TestMultiSelectRejectsOutOfRange(t *testing.T) {
	var out strings.Builder
	_, err := New(strings.NewReader("4\n"), &out).MultiSelect("Pick many", testOptions(), nil)
	if err == nil {
		t.Fatal("MultiSelect accepted out-of-range input")
	}
}

func TestMultiSelectDefaultsOnEmpty(t *testing.T) {
	var out strings.Builder
	got, err := New(strings.NewReader("\n"), &out).MultiSelect("Pick many", testOptions(), []bool{true, false, true})
	if err != nil {
		t.Fatalf("MultiSelect returned error: %v", err)
	}
	want := []int{0, 2}
	if !sameInts(got, want) {
		t.Fatalf("MultiSelect() = %v, want %v", got, want)
	}
}

func TestMultiSelectRendersCheckboxDefaults(t *testing.T) {
	var out strings.Builder
	_, err := New(strings.NewReader("\n"), &out).MultiSelect("Pick many", testOptions(), []bool{true, false, true})
	if err != nil {
		t.Fatalf("MultiSelect returned error: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"[x] [1] Alpha",
		"[ ] [2] Beta",
		"[x] [3] Gamma",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("MultiSelect output = %q, want contains %q", got, want)
		}
	}
}

func testOptions() []Option {
	return []Option{
		{Label: "Alpha", Help: "first"},
		{Label: "Beta", Help: "second"},
		{Label: "Gamma"},
	}
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
