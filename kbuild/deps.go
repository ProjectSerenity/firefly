// Check for tools we depend on.

package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"golang.org/x/mod/semver"
)

// CheckDeps ensures we have the dependencies we
// need, or it exits with a suitable error message.
//
func (ctx *Context) CheckDeps() {
	checks := []func() string{
		ctx.checkObjcopy,
		ctx.checkXorriso,
		ctx.checkGrubMkrescue,
		ctx.checkNasm,
	}

	var failed []string
	for _, check := range checks {
		msg := check()
		if msg == "" {
			continue
		}

		failed = append(failed, msg)
	}

	if len(failed) == 0 {
		return
	}

	ctx.Fatalf("missing at least one dependency:\n\t%s", strings.Join(failed, "\n\t"))
}

// checkObjcopy checks for objcopy, with version
// 2.26.0 or higher.
//
func (ctx *Context) checkObjcopy() string {
	var err error
	ctx.objcopy, err = exec.LookPath("objcopy")
	if err != nil {
		return "`objcopy` could not be found: install `binutils`"
	}

	msg, err := exec.Command(ctx.objcopy, "-V").CombinedOutput()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			ctx.Errorf("%s", e.Stderr)
		}

		return fmt.Sprintf("`objcopy`: failed to determine version: %v", err)
	}

	return checkObjcopyVersion(msg)
}

func checkObjcopyVersion(msg []byte) string {
	const want = "v2.26.0"

	// Sample output:
	//
	// 	GNU objcopy (GNU Binutils for Ubuntu) 2.34
	// 	Copyright (C) 2020 Free Software Foundation, Inc.
	// 	This program is free software; you may redistribute it under the terms of
	// 	the GNU General Public License version 3 or (at your option) any later version.
	// 	This program has absolutely no warranty.

	i := bytes.IndexByte(msg, '\n')
	if i >= 0 {
		msg = msg[:i]
	}

	i = bytes.LastIndexByte(msg, ' ')
	if i >= 0 {
		msg = msg[i:]
	}

	version := "v" + strings.TrimSpace(string(msg))
	if !semver.IsValid(version) {
		return fmt.Sprintf("`objcopy`: failed to detect version: %q is not a valid version", version)
	}

	if semver.Compare(version, want) < 0 {
		return fmt.Sprintf("`objcopy`: version %s is older than %s: install newer `binutils`", version, want)
	}

	return ""
}

// checkXorriso checks for xorriso, with version
// 1.5.0 or higher.
//
func (ctx *Context) checkXorriso() string {
	var err error
	ctx.xorriso, err = exec.LookPath("xorriso")
	if err != nil {
		return "`xorriso` could not be found: install `xorriso`"
	}

	msg, err := exec.Command(ctx.xorriso, "--version").CombinedOutput()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			ctx.Errorf("%s", e.Stderr)
		}

		return fmt.Sprintf("`xorriso`: failed to determine version: %v", err)
	}

	return checkXorrisoVersion(msg)
}

func checkXorrisoVersion(msg []byte) string {
	const want = "v1.5.0"

	// Sample output:
	//
	// 	xorriso 1.5.2 : RockRidge filesystem manipulator, libburnia project.
	//
	// 	xorriso 1.5.2
	// 	ISO 9660 Rock Ridge filesystem manipulator and CD/DVD/BD burn program
	// 	Copyright (C) 2019, Thomas Schmitt <scdbackup@gmx.net>, libburnia project.
	// 	xorriso version   :  1.5.2
	// 	Version timestamp :  2019.10.26.180001
	// 	Build timestamp   :  -none-given-
	// 	libisofs   in use :  1.5.2  (min. 1.5.2)
	// 	libjte     in use :  2.0.0  (min. 2.0.0)
	// 	libburn    in use :  1.5.2  (min. 1.5.2)
	// 	libburn OS adapter:  internal GNU/Linux SG_IO adapter sg-linux
	// 	libisoburn in use :  1.5.2  (min. 1.5.2)
	// 	Provided under GNU GPL version 3 or later, due to libreadline license.
	// 	There is NO WARRANTY, to the extent permitted by law.

	msg = bytes.TrimPrefix(msg, []byte("xorriso "))
	i := bytes.IndexByte(msg, ' ')
	if i >= 0 {
		msg = msg[:i]
	}

	version := "v" + strings.TrimSpace(string(msg))
	if !semver.IsValid(version) {
		return fmt.Sprintf("`xorriso`: failed to detect version: %q is not a valid version", version)
	}

	if semver.Compare(version, want) < 0 {
		return fmt.Sprintf("`xorriso`: version %s is older than %s: install newer `xorriso`", version, want)
	}

	return ""
}

// checkGrubMkrescue checks for GRUB's mkrescue tool.
//
func (ctx *Context) checkGrubMkrescue() string {
	var err error
	ctx.grubMkrescue, err = exec.LookPath("grub-mkrescue")
	if err != nil {
		return "`grub-mkrescue` could not be found: install `grub-pc-bin`"
	}

	err = exec.Command(ctx.grubMkrescue, "--version").Run()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			ctx.Errorf("%s", e.Stderr)
		}

		return fmt.Sprintf("`grub-mkrescue`: failed to determine version: %v", err)
	}

	return ""
}

// checkNasm checks for the network assembler.
//
func (ctx *Context) checkNasm() string {
	var err error
	ctx.nasm, err = exec.LookPath("nasm")
	if err != nil {
		return "`nasm` could not be found: install `nasm`"
	}

	err = exec.Command(ctx.nasm, "--version").Run()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			ctx.Errorf("%s", e.Stderr)
		}

		return fmt.Sprintf("`nasm`: failed to determine version: %v", err)
	}

	return ""
}
