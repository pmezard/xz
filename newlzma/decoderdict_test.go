package newlzma

import (
	"fmt"
	"testing"
)

func peek(d *decoderDict) []byte {
	p := make([]byte, d.Buffered())
	k, err := d.Peek(p)
	if err != nil {
		panic(fmt.Errorf("peek: "+
			"Read returned unexpected error %s", err))
	}
	if k != len(p) {
		panic(fmt.Errorf("peek: "+
			"Read returned %d; wanted %d", k, len(p)))
	}
	return p
}

func TestInitDecoderDict(t *testing.T) {
	var d decoderDict
	if err := initDecoderDict(&d, 0, 0); err == nil {
		t.Fatalf("no error for zero dictionary capacity")
	}
	if err := initDecoderDict(&d, 1, 0); err == nil {
		t.Fatalf("no error for bufCap < dictCap")
	}
	if err := initDecoderDict(&d, 8, 12); err != nil {
		t.Fatalf("error %s", err)
	}
}

func TestDecoderDict(t *testing.T) {
	var d decoderDict
	if err := initDecoderDict(&d, 8, 12); err != nil {
		t.Fatalf("initDecoderDict error %s", err)
	}
	if err := d.WriteByte('a'); err != nil {
		t.Fatalf("WriteByte error %s", err)
	}
	if err := d.WriteByte('b'); err != nil {
		t.Fatalf("WriteByte error %s", err)
	}
	if err := d.WriteByte('c'); err != nil {
		t.Fatalf("WriteByte error %s", err)
	}
	if err := d.WriteByte('d'); err != nil {
		t.Fatalf("WriteByte error %s", err)
	}
	err := d.WriteMatch(4, 5)
	if err != nil {
		t.Fatalf("WriteMatch error %s", err)
	}
	s := string(peek(&d))
	if s != "abcdabcda" {
		t.Fatalf("WriteMatch produced buffer content %q; want %q",
			s, "abcdabcda")
	}
	if d.Len() != d.capacity {
		t.Fatalf("dictionary length is %d; want capacity %d",
			d.Len(), d.capacity)
	}
	c := d.ByteAt(10)
	if c != 0 {
		t.Fatalf("d.ByteAt(10) returned %c; want %c", c, 0)
	}
	c = d.ByteAt(2)
	if c != 'd' {
		t.Fatalf("d.ByteAt(2) returned %c; want %c", c, 'd')
	}
	p := make([]byte, 7)
	n, err := d.Read(p)
	if err != nil {
		t.Fatalf("Read error %s", err)
	}
	if n != 7 {
		t.Fatalf("Read returned %d; want %d", n, 7)
	}
	if string(p) != "abcdabc" {
		t.Fatalf("Read returned %q; want %q", p, "abcdabc")
	}
	s = string(peek(&d))
	if s != "da" {
		t.Fatalf("Read produced buffer %q; want %q", s, "da")
	}
	err = d.WriteMatch(3, 3)
	if err != nil {
		t.Fatalf("WriteMatch error %s", err)
	}
	p = make([]byte, 8)
	n, err = d.Read(p)
	if err != nil {
		t.Fatalf("Read#2 error %s", err)
	}
	if n != 5 {
		t.Fatalf("Read#2 returned %d; want %d", n, 5)
	}
	p = p[:n]
	if string(p) != "dacda" {
		t.Fatalf("Read#2 returned %q; want %q", p, "dacda")
	}
	n = d.Buffered()
	if n != 0 {
		t.Fatalf("Buffered returned %d after Read#2; want %d",
			n, 0)
	}
}