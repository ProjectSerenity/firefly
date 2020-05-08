// Command imgbuild builds the firefly disk image.
//
package main

import (
	"bytes"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
)

func init() {
	log.SetFlags(0)
}

func WriteAt(buf []byte, at, maxSize int64, name string) (end int64) {
	f, err := os.Open(name)
	if err != nil {
		log.Fatalf("failed to open %s: %v", name, err)
	}

	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		log.Fatalf("failed to stat %s: %v", name, err)
	}

	size := info.Size()
	length := int64(len(buf))
	if size > maxSize {
		log.Fatalf("%s is too large: %d bytes exceeds %d limit", name, size, maxSize)
	}

	if at > length || size > length || at+size > length {
		log.Fatalf("buffer too small for %s: %d bytes at index %d to buffer of size %d", name, size, at, length)
	}

	w := bytes.NewBuffer(buf[at:at])
	n, err := io.Copy(w, f)
	if err != nil {
		log.Fatalf("failed to copy %s: %v", name, err)
	}

	if n != size {
		log.Fatalf("failed to copy %s: wrote %d, want %d", name, n, size)
	}

	end = at + n

	return end
}

func main() {
	var kernel, loader, mbr, out string
	flag.StringVar(&kernel, "kernel", "", "path to kernel binary")
	flag.StringVar(&loader, "loader", "", "path to boot loader")
	flag.StringVar(&mbr, "mbr", "", "path to master boot loader")
	flag.StringVar(&out, "o", "", "path to write output")
	flag.Parse()

	if mbr == "" {
		log.Fatalf("-mbr not specified")
	}

	if loader == "" {
		log.Fatalf("-loader not specified")
	}

	if kernel == "" {
		log.Fatalf("-kernel not specified")
	}

	if out == "" {
		log.Fatalf("-o not specified")
	}

	data := make([]byte, 128*1<<20)
	WriteAt(data, 0, 512, mbr)
	end := WriteAt(data, 16*512, 4*1024, loader)
	WriteAt(data, end, 28*1024, kernel)

	err := ioutil.WriteFile(out, data, 0666)
	if err != nil {
		log.Fatalf("failed to write %s: %v", out, err)
	}
}
