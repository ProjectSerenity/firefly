package main

import (
	"strings"
	"testing"
)

func TestCheckObjcopyVersion(t *testing.T) {
	tests := []struct {
		Lines []string
		Want  string
	}{
		{
			Lines: []string{
				"GNU objcopy (GNU Binutils for Ubuntu) 2.34",
				"Copyright (C) 2020 Free Software Foundation, Inc.",
				"This program is free software; you may redistribute it under the terms of",
				"the GNU General Public License version 3 or (at your option) any later version.",
				"This program has absolutely no warranty.",
			},
			Want: "",
		},
		{
			Lines: []string{
				"GNU objcopy (GNU Binutils for Ubuntu) 2.26",
				"Copyright (C) 2020 Free Software Foundation, Inc.",
				"This program has absolutely no warranty.",
			},
			Want: "",
		},
		{
			Lines: []string{
				"GNU objcopy (GNU Binutils for Ubuntu) 2.25.99",
				"Copyright (C) 2020 Free Software Foundation, Inc.",
				"This program has absolutely no warranty.",
			},
			Want: "`objcopy`: version v2.25.99 is older than v2.26.0: install newer `binutils`",
		},
		{
			Lines: []string{
				"GNU objcopy (GNU Binutils for Ubuntu)",
				"Copyright (C) 2020 Free Software Foundation, Inc.",
				"This program has absolutely no warranty.",
			},
			Want: "`objcopy`: failed to detect version: \"vUbuntu)\" is not a valid version",
		},
	}

	for i, test := range tests {
		got := checkObjcopyVersion([]byte(strings.Join(test.Lines, "\n")))
		if got != test.Want {
			t.Errorf("#%d: checkObjcopyVersion() bad output:\n\tGot:  %q\n\tWant: %q", i, got, test.Want)
		}
	}
}

func TestCheckXorrisoVersion(t *testing.T) {
	tests := []struct {
		Lines []string
		Want  string
	}{
		{
			Lines: []string{
				"xorriso 1.5.2 : RockRidge filesystem manipulator, libburnia project.",
				"",
				"xorriso 1.5.2",
				"ISO 9660 Rock Ridge filesystem manipulator and CD/DVD/BD burn program",
				"Copyright (C) 2019, Thomas Schmitt <scdbackup@gmx.net>, libburnia project.",
				"xorriso version   :  1.5.2",
				"Version timestamp :  2019.10.26.180001",
				"Build timestamp   :  -none-given-",
				"libisofs   in use :  1.5.2  (min. 1.5.2)",
				"libjte     in use :  2.0.0  (min. 2.0.0)",
				"libburn    in use :  1.5.2  (min. 1.5.2)",
				"libburn OS adapter:  internal GNU/Linux SG_IO adapter sg-linux",
				"libisoburn in use :  1.5.2  (min. 1.5.2)",
				"Provided under GNU GPL version 3 or later, due to libreadline license.",
				"There is NO WARRANTY, to the extent permitted by law.",
			},
			Want: "",
		},
		{
			Lines: []string{
				"xorriso 1.5.0 : RockRidge filesystem manipulator, libburnia project.",
			},
			Want: "",
		},
		{
			Lines: []string{
				"xorriso 1.4.99 : RockRidge filesystem manipulator, libburnia project.",
			},
			Want: "`xorriso`: version v1.4.99 is older than v1.5.0: install newer `xorriso`",
		},
		{
			Lines: []string{
				"xorriso: RockRidge filesystem manipulator, libburnia project.",
			},
			Want: "`xorriso`: failed to detect version: \"vxorriso:\" is not a valid version",
		},
	}

	for i, test := range tests {
		got := checkXorrisoVersion([]byte(strings.Join(test.Lines, "\n")))
		if got != test.Want {
			t.Errorf("#%d: checkXorrisoVersion() bad output:\n\tGot:  %q\n\tWant: %q", i, got, test.Want)
		}
	}
}
