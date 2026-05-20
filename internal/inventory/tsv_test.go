package inventory

import (
	"strings"
	"testing"
)

func TestParseTSV(t *testing.T) {
	input := "# comment\n\nfront-door\t192.168.1.10\t80\ngarage\t10.0.0.5\t8080\n"
	cameras, err := ParseTSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cameras) != 2 {
		t.Fatalf("expected 2 cameras, got %d", len(cameras))
	}
	if cameras[0].Name != "front-door" || cameras[0].IP != "192.168.1.10" || cameras[0].Port != 80 {
		t.Errorf("unexpected row 0: %+v", cameras[0])
	}
	if cameras[1].Name != "garage" || cameras[1].Port != 8080 {
		t.Errorf("unexpected row 1: %+v", cameras[1])
	}
}

func TestParseTSV_BadPort(t *testing.T) {
	_, err := ParseTSV(strings.NewReader("cam\t192.168.1.1\tnotaport\n"))
	if err == nil {
		t.Fatal("expected error for bad port")
	}
}

func TestParseTSV_WrongColumns(t *testing.T) {
	_, err := ParseTSV(strings.NewReader("cam\t192.168.1.1\n"))
	if err == nil {
		t.Fatal("expected error for wrong column count")
	}
}
