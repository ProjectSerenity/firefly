package build

import (
	"reflect"
	"strings"
	"testing"
)

func TestGoMajorMinorVersion(t *testing.T) {
	tests := []struct {
		V       string
		Want    string
		Invalid bool
	}{
		{
			V:    "go1.2.3",
			Want: "go1.2",
		},
		{
			V:    "go11.22.33",
			Want: "go11.22",
		},
		{
			V:    "go1.2",
			Want: "go1.2",
		},
		{
			V:       "v1.2.3",
			Invalid: true,
		},
	}

	for _, test := range tests {
		t.Run(test.V, func(t *testing.T) {
			got, ok := goMajorMinorVersion(test.V)
			if test.Invalid {
				if ok {
					t.Fatalf("goMajorMinorVersion(%q): got %q, want invalid", test.V, got)
				}

				return
			}

			if !ok {
				t.Fatalf("goMajorMinorVersion(%q): got invalid, want %s", test.V, test.Want)
			}

			if got != test.Want {
				t.Fatalf("goMajorMinorVersion(%q): got %s, want %s", test.V, got, test.Want)
			}
		})
	}
}

func TestOverrideEnv(t *testing.T) {
	tests := []struct {
		Environ   []string
		Overrides []string
		Want      []string
	}{
		{
			Environ: []string{
				"A=b",
				"B=c",
			},
			Overrides: []string{
				"A=d",
			},
			Want: []string{
				"A=d",
				"B=c",
			},
		},
		{
			Environ: []string{
				"A=b",
				"B=c",
			},
			Overrides: []string{
				"D=d",
			},
			Want: []string{
				"A=b",
				"B=c",
				"D=d",
			},
		},
	}

	for i, test := range tests {
		// Make a copy of environ to make
		// sure we can detect changes to
		// the original.
		orig := make([]string, len(test.Environ))
		copy(orig, test.Environ)

		got := OverrideEnv(test.Environ, test.Overrides...)
		if !reflect.DeepEqual(got, test.Want) {
			t.Errorf("#%d: overrides mismatch:\n\t Got:\n\t\t%s\n\tWant:\n\t\t%s",
				i+1, strings.Join(got, "\n\t\t"), strings.Join(test.Want, "\n\t\t"))
		}

		if !reflect.DeepEqual(test.Environ, orig) {
			t.Errorf("#%d: environ overridden:\n\t Had:\n\t\t%s\n\tHave:\n\t\t%s",
				i+1, strings.Join(orig, "\n\t\t"), strings.Join(test.Environ, "\n\t\t"))
		}
	}
}
