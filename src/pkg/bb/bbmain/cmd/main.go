// Copyright 2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package main is the busybox main.go template.
package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-root/gobusybox/src/pkg/bb/bbmain"
	// There MUST NOT be any other dependencies here.
	//
	// It is preferred to copy minimal code necessary into this file, as
	// dependency management for this main file is... hard.
)

// AbsSymlink returns an absolute path for the link from a file to a target.
func AbsSymlink(originalFile, target string) string {
	if !filepath.IsAbs(originalFile) {
		var err error
		originalFile, err = filepath.Abs(originalFile)
		if err != nil {
			// This should not happen on Unix systems, or you're
			// already royally screwed.
			log.Fatalf("could not determine absolute path for %v: %v", originalFile, err)
		}
	}
	// Relative symlinks are resolved relative to the original file's
	// parent directory.
	//
	// E.g. /bin/defaultsh -> ../bbin/elvish
	if !filepath.IsAbs(target) {
		return filepath.Join(filepath.Dir(originalFile), target)
	}
	return target
}

// IsTargetSymlink returns true if a target of a symlink is also a symlink.
func IsTargetSymlink(originalFile, target string) bool {
	s, err := os.Lstat(AbsSymlink(originalFile, target))
	if err != nil {
		return false
	}
	return (s.Mode() & os.ModeSymlink) == os.ModeSymlink
}

// ResolveUntilLastSymlink resolves until the last symlink.
//
// This is needed when we have a chain of symlinks and want the last
// symlink, not the file pointed to (which is why we don't use
// filepath.EvalSymlinks)
//
// I.e.
//
// /foo/bar -> ../baz/foo
// /baz/foo -> bla
//
// ResolveUntilLastSymlink(/foo/bar) returns /baz/foo.
func ResolveUntilLastSymlink(p string) string {
	for target, err := os.Readlink(p); err == nil && IsTargetSymlink(p, target); target, err = os.Readlink(p) {
		p = AbsSymlink(p, target)
	}
	return p
}

func run() {
	name := filepath.Base(os.Args[0])
	if err := bbmain.Run(name); err != nil {
		log.Fatalf("%s: %v", name, err)
	}
}

func main() {
	os.Args[0] = ResolveUntilLastSymlink(os.Args[0])

	run()
}

// u-root was originally built around the use of symlinks, but not all systems
// have symlinks. This only recently became an issue with the Plan 9 port.
//
// One way to get around this lack, inefficiently, is to make each of the symlinks
// a small shell script, e.g., on Plan 9, one might have, in /bbin/ls,
// #!/bin/rc
// bb ls
// This leaves a lot to be desired: it puts the execution of a shell in front
// of each u-root command, and it requires the existence of that shell on the
// system.
//
// The goal is that a single u-root file lead to running the u-root busybox
// with no intermediate programs running.
//
// It is worth taking a look at what a symlink is, how it works in operation,
// and how we might achieve the same goal some other way.
//
// A symlink is plain file, containing 0 or more bytes of text (or utf-8, depending)
// with an attribute that causes the kernel to give it special treatment.
// It is not available on all file systems.
//
// [Note: they were invented in 1965 for Multics].
// The symlink is itself still controversial, though widely used.
//
// Consider the process of traversing a symlink: it involves the equivalent
// of stat, open, read, evaluate contents, use that as a file name, repeat as needed.
//
// It is possible to get that same effect, with the same overheads, by using #!
// files but specifying bb as the interpreter.
//
// ls would then be:
// #!/bin/bb ls
//
// Note that the absolute path is required, else Linux will throw an error as bb
// is not in the list of allowed interpreters.
// The /bin/bb path is not an issue on Plan 9, since users construct their name space
// on startup and binding /bbin into /bin is no problem.
//
// In this case the kernel will stat, open, and read the file, find the executable name,
// and start it. This approach has as low overhead as the symlink approach.
//
// One problem remains: Unix and Plan 9 evaluate arguments in a #! file differently,
// and, further, invoke the argument in a different way.
// Given the file shown above, bb on Plan9 gets the arguments:
// [ls ls /tmp/ls]
// With the same file, bb on Linux gets this:
// [/bbin/bb ls /tmp/ls]
// But wait! There's more!
// On Plan 9, the arguments following the interpreter are tokenized (split on space)
// and on Linux, they are not.
//
// This leads to a few conclusions:
// - We can get around lack of symlinks by using #! (sh-bang) files with an absolute path to
//   bb as the interpreter, e.g. #!/abs/path/to/bb argument.
//   This achieves the "exec once" goal.
// - We can specify which u-root tool to use via arguments to bb in the #! file.
// - The argument to the interpreter (/bbin/bb) should be one token (e.g. ls) because of different
//   behavior in different systems (some tokenize, some do not).
// - Because of the differences in how arguments are presented to #! on different kernels,
//   there should be a reasonably unique marker so that bb can have confidence that
//   it is running as an interpreter.
//
// The conclusions lead to the following design:
// #! files for bb specify their argument with #!. E.g., the file for ls looks like this:
// #!/bbin/bb #!ls
// On Linux, the args to bb then look like:
// [/bbin/bb #!ls /tmp/ls ...]
// on Plan 9:
// [ls #!ls /tmp/ls ...]
// The code needs to change the arguments to look like an exec:
// [/tmp/ls ...]
// In each case, the second arg begins with a #!, which is extremely unlikely to appear
// in any other context (save testing #! files, of course).
// The result is that the kernel, given a path to a u-root #! file, will read that file,
// then exec bbin with the argument from the #! and any additional arguments from the exec.
// The overhead in this case is no more than the symlink overhead.
// A final advantage is that we can now install u-root on file systems that don't have
// symbolic links, e.g. VFAT, and it will have low overhead.
//
// So, dear reader, if you are wondering why the little bit of code below is the way
// it is, now you know.
func init() {
	// If this has been run from a #! file, it will have at least
	// 3 args, and os.Args needs to be reconstructed.
	if len(os.Args) > 2 && strings.HasPrefix(os.Args[1], "#!") {
		os.Args = os.Args[2:]
	}
	m := func() {
		if len(os.Args) == 1 {
			log.Fatalf("Invalid busybox command: %q", os.Args)
		}
		// Use argv[1] as the name.
		os.Args = os.Args[1:]
		run()
	}
	bbmain.Register("bbdiagnose", bbmain.Noop, bbmain.ListCmds)
	bbmain.RegisterDefault(bbmain.Noop, m)
}
