package media

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, name string, data []byte) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProbeAIFF(t *testing.T) {
	comm := new(bytes.Buffer)
	comm.WriteString("COMM")
	binary.Write(comm, binary.BigEndian, uint32(18))
	binary.Write(comm, binary.BigEndian, uint16(2))              // channels
	binary.Write(comm, binary.BigEndian, uint32(44100))          // sample frames
	binary.Write(comm, binary.BigEndian, uint16(16))             // bits
	comm.Write([]byte{0x40, 0x0e, 0xac, 0x44, 0, 0, 0, 0, 0, 0}) // 44100.0 ext80

	body := bytes.NewBuffer(nil)
	body.WriteString("AIFF")
	body.Write(comm.Bytes())

	file := bytes.NewBuffer(nil)
	file.WriteString("FORM")
	binary.Write(file, binary.BigEndian, uint32(body.Len()))
	file.Write(body.Bytes())

	info := Probe(writeTemp(t, "tone.aiff", file.Bytes()))
	if info.SampleRate != 44100 || info.Channels != 2 {
		t.Fatalf("unexpected info: %+v", info)
	}
	if info.Duration < 0.99 || info.Duration > 1.01 {
		t.Fatalf("duration = %v, want 1s", info.Duration)
	}
}

func TestProbeAAC(t *testing.T) {
	frame := func() []byte {
		payload := make([]byte, 10)
		h := []byte{0xFF, 0xF1, 0x50, 0x80, 0x02, 0x3F, 0xFC}
		return append(h, payload...)
	}
	data := append(frame(), frame()...)
	info := Probe(writeTemp(t, "tone.aac", data))
	if info.SampleRate != 44100 || info.Channels != 2 {
		t.Fatalf("unexpected info: %+v", info)
	}
	want := 2 * 1024.0 / 44100.0
	if info.Duration < want-0.001 || info.Duration > want+0.001 {
		t.Fatalf("duration = %v, want %v", info.Duration, want)
	}
}

func TestProbeWMA(t *testing.T) {
	guid := []byte{0x30, 0x26, 0xB2, 0x75, 0x8E, 0x66, 0xCF, 0x11, 0xA6, 0xD9, 0x00, 0xAA, 0x00, 0x62, 0xCE, 0x6C}
	propsGUID := []byte{0xA1, 0xDC, 0xAB, 0x8C, 0x47, 0xA9, 0xCF, 0x11, 0x8E, 0xE4, 0x00, 0xC0, 0x0C, 0x20, 0x53, 0x65}
	obj := new(bytes.Buffer)
	obj.Write(propsGUID)
	binary.Write(obj, binary.LittleEndian, uint64(24+80))
	data := make([]byte, 80)
	binary.LittleEndian.PutUint64(data[40:48], 600_000_000) // 60s in 100ns units
	binary.LittleEndian.PutUint64(data[56:64], 3000)        // 3s preroll (ms)
	obj.Write(data)

	file := new(bytes.Buffer)
	file.Write(guid)
	binary.Write(file, binary.LittleEndian, uint64(30+obj.Len()))
	binary.Write(file, binary.LittleEndian, uint32(1))
	file.Write([]byte{0x01, 0x02})
	file.Write(obj.Bytes())

	info := Probe(writeTemp(t, "tone.wma", file.Bytes()))
	if info.Duration < 56.9 || info.Duration > 57.1 {
		t.Fatalf("duration = %v, want 57s", info.Duration)
	}
}
