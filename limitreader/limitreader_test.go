package limitreader

import (
	"bytes"
	"testing"
)

func TestLimitReader(t *testing.T) {
	databuf := bytes.NewBuffer([]byte("helloworld"))
	f := New(databuf, 5)

	buf := make([]byte, 1000)
	n, err := f.Read(buf)
	if err != nil {
		t.Fatalf("f.Read(buf) = err: %v", err)
	}

	if n != 5 {
		t.Errorf("f.Read(buf) = %d want %d [byte(s) read]", n, 5)
	}

	if string(buf[:n]) != "hello" {
		t.Errorf("buf = %v; want initial %d byte(s) %q", buf, n, "hello")
	}
}
