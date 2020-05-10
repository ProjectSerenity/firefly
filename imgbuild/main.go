// Command imgbuild builds the firefly disk image.
//
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
)

func init() {
	log.SetFlags(0)
}

func Open(name string) (f *os.File, size int64) {
	var err error
	f, err = os.Open(name)
	if err != nil {
		log.Fatalf("failed to open %s: %v", name, err)
	}

	info, err := f.Stat()
	if err != nil {
		log.Fatalf("failed to stat %s: %v", name, err)
	}

	size = info.Size()

	return f, size
}

func WriteFile(buf []byte, at, size int64, name string, f *os.File) {
	w := bytes.NewBuffer(buf[at:at])
	n, err := io.Copy(w, f)
	if err != nil {
		log.Fatalf("failed to copy %s: %v", name, err)
	}

	if n != size {
		log.Fatalf("failed to copy %s: wrote %d, want %d", name, n, size)
	}
}

func WriteAt(buf []byte, at, wantSize int64, name string) {
	f, size := Open(name)
	defer f.Close()

	if size != wantSize {
		log.Fatalf("%s: got %d bytes, want %d", name, size, wantSize)
	}

	WriteFile(buf, at, size, name, f)
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

	const (
		SizeImg         = 128 * 1 << 20 // 128 MiB
		StartMBR        = 0
		SizeMBR         = 512
		StartLoader     = 16 * 512
		SizeLoader      = 4 * 1024
		StartKernelSize = StartLoader + SizeLoader - 4
		SizeKernelSize  = 4
		StartKernel     = StartKernelSize + SizeKernelSize
	)

	data := make([]byte, SizeImg)
	WriteAt(data, StartMBR, SizeMBR, mbr)
	WriteAt(data, StartLoader, SizeLoader, loader)

	f, size := Open(kernel)
	defer f.Close()

	binary.LittleEndian.PutUint32(data[StartKernelSize:], uint32(size)/512+1) // Number of 512-byte blocks.
	WriteFile(data, StartKernel, size, kernel, f)

	err := ioutil.WriteFile(out, data, 0666)
	if err != nil {
		log.Fatalf("failed to write %s: %v", out, err)
	}
}
