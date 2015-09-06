// Copyright 2015 Rob Pike. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Scrub reads a JPG file and copies it to standard output
// after deleting any App, JPEG, or comment segment. That is,
// it scrubs all metadata from the input and writes the result
// to standard output.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

var iFlag = flag.Bool("i", false, "overwrite the input in place")

func main() {
	log.SetPrefix("scrub: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()
	switch len(flag.Args()) {
	case 0:
		if *iFlag {
			log.Fatal("cannot overwrite standard input")
		}
		scrub(os.Stdin)
	case 1:
		file := flag.Arg(0)
		f, err := os.Open(file)
		ck(err)
		scrub(f)
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: scrub [[-i] file]\n")
	os.Exit(2)
}

const (
	/* Constants all preceded by byte 0xFF */
	SOF  = 0xC0 /* Start of Frame */
	SOF2 = 0xC2 /* Start of Frame; progressive Huffman */
	JPG  = 0xC8 /* Reserved for JPEG extensions */
	DHT  = 0xC4 /* Define Huffman Tables */
	DAC  = 0xCC /* Arithmetic coding conditioning */
	RST  = 0xD0 /* Restart interval termination */
	RST7 = 0xD7 /* Restart interval termination (highest value) */
	SOI  = 0xD8 /* Start of Image */
	EOI  = 0xD9 /* End of Image */
	SOS  = 0xDA /* Start of Scan */
	DQT  = 0xDB /* Define quantization tables */
	DNL  = 0xDC /* Define number of lines */
	DRI  = 0xDD /* Define restart interval */
	DHP  = 0xDE /* Define hierarchical progression */
	EXP  = 0xDF /* Expand reference components */
	APPn = 0xE0 /* Reserved for application segments */
	JPGn = 0xF0 /* Reserved for JPEG extensions */
	COM  = 0xFE /* Comment */
)

func scrub(f *os.File) {
	data, err := ioutil.ReadAll(f)
	ck(err)
	s := NewScanner(data)
	s.header()
	for s.segment() > 0 {
	}
	if *iFlag {
		ck(ioutil.WriteFile(flag.Arg(0), s.out, 0664))
	} else {
		os.Stdout.Write(s.out)
	}
}

func ck(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

type Scanner struct {
	in     []byte
	out    []byte
	offset int
}

func NewScanner(data []byte) *Scanner {
	return &Scanner{in: data, out: make([]byte, 0, len(data))}
}

func (s *Scanner) ReadByte() int {
	if len(s.in) == 0 {
		log.Fatal("EOF")
	}
	s.out = append(s.out, s.in[0])
	c := s.in[0]
	s.in = s.in[1:]
	s.offset++
	return int(c)
}

func (s *Scanner) Read(n int) (data []byte) {
	if len(s.in) < n {
		log.Fatal("EOF")
	}
	data, s.in = s.in[0:n], s.in[n:]
	s.out = append(s.out, data...)
	s.offset += n
	return
}

func (s *Scanner) drain() {
	s.out = append(s.out, s.in...)
	// s.in no longer valid
}

func (s *Scanner) header() {
	if c := s.marker(); c != SOI {
		log.Fatalf("expected SOI; saw 0x%.2x\n", c)
	}
}

func (s *Scanner) marker() int {
	var c int
	for {
		c = s.ReadByte()
		if c != 0 {
			break
		}
		fmt.Fprintf(os.Stderr, "scrub: skipping zero byte\n")
	}
	if c != 0xFF {
		log.Fatalf("expecting marker at 0x%x, found 0x%.2x", s.offset-1, c)
	}
	for c == 0xFF {
		c = s.ReadByte()
	}
	return c
}

func int2(b []byte) int {
	return int(b[0])<<8 + int(b[1])
}

func (s *Scanner) segment() int {
	start := len(s.out)
	var c int
	switch c = s.marker(); c {
	case EOI:
		return 0
	case 0:
		log.Fatalf("expecting marker; saw 0x%.2x at offset 0x%x", c, s.offset-1)
	}
	buf := s.Read(2)
	n := int2(buf[0:2])
	if n < 2 {
		log.Fatal("early EOF")
	}
	n -= 2
	buf = s.Read(n)
	// Is this an App, JPEG, or comment segment? if so, ignore it
	if c >= APPn {
		s.out = s.out[0:start]
	}
	if c == SOS {
		// This is real data; just run to completion
		s.drain()
		return 0
	}
	return c
}
