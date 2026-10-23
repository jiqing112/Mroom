package media

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// Info is the technical description of an audio file.
type Info struct {
	Duration   float64 // seconds
	Bitrate    int     // kbps
	SampleRate int
	Channels   int
}

var supportedExt = map[string]string{
	".mp3":  "audio/mpeg",
	".flac": "audio/flac",
	".m4a":  "audio/mp4",
	".m4b":  "audio/mp4",
	".mp4":  "audio/mp4",
	".aac":  "audio/aac",
	".ogg":  "audio/ogg",
	".oga":  "audio/ogg",
	".opus": "audio/ogg",
	".wav":  "audio/wav",
	".wma":  "audio/x-ms-wma",
	".aiff": "audio/aiff",
	".aif":  "audio/aiff",
}

func ContentType(path string) (string, bool) {
	ct, ok := supportedExt[strings.ToLower(filepath.Ext(path))]
	return ct, ok
}

func IsAudio(path string) bool {
	_, ok := supportedExt[strings.ToLower(filepath.Ext(path))]
	return ok
}

// Probe reads just enough of the file to determine duration/bitrate without
// pulling large payloads into memory.
func Probe(path string) Info {
	f, err := os.Open(path)
	if err != nil {
		return Info{}
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return Info{}
	}
	size := st.Size()
	ext := strings.ToLower(filepath.Ext(path))

	var info Info
	switch ext {
	case ".mp3":
		info = probeMP3(f, size)
	case ".flac":
		info = probeFLAC(f, size)
	case ".ogg", ".oga", ".opus":
		info = probeOgg(f, size)
	case ".wav":
		info = probeWav(f, size)
	case ".aiff", ".aif":
		info = probeAIFF(f, size)
	case ".aac":
		info = probeAAC(f, size)
	case ".wma":
		info = probeWMA(f, size)
	case ".m4a", ".m4b", ".mp4":
		info = probeMP4(f, size)
	}
	if info.Bitrate == 0 && info.Duration > 0 {
		info.Bitrate = int(float64(size) * 8 / info.Duration / 1000)
	}
	if info.Duration < 0 {
		info.Duration = 0
	}
	return info
}

// ---------------------------------------------------------------- mp3

var mp3Bitrates = [5][16]int{
	{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}, // MPEG1 L3
	{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},     // MPEG2/2.5 L3
}

func probeMP3(f *os.File, size int64) Info {
	head := make([]byte, 10)
	if _, err := f.ReadAt(head, 0); err != nil {
		return Info{}
	}
	start := int64(0)
	if string(head[0:3]) == "ID3" {
		sz := int64(head[6]&0x7f)<<21 | int64(head[7]&0x7f)<<14 | int64(head[8]&0x7f)<<7 | int64(head[9]&0x7f)
		start = 10 + sz
		if head[5]&0x10 != 0 {
			start += 10
		}
	}

	buf := make([]byte, 128*1024)
	n, _ := f.ReadAt(buf, start)
	buf = buf[:n]

	frame := -1
	for i := 0; i+4 < len(buf); i++ {
		if buf[i] != 0xFF || buf[i+1]&0xE0 != 0xE0 {
			continue
		}
		if br, sr, spf, flen := parseMP3Header(buf[i:]); br > 0 && flen > 0 {
			info := Info{Bitrate: br, SampleRate: sr}
			// Xing / Info / VBRI header carries the exact frame count.
			frames := xingFrames(buf[i:])
			if frames > 0 {
				info.Duration = float64(frames) * float64(spf) / float64(sr)
				return info
			}
			if size > start {
				info.Duration = float64(size-start) * 8 / float64(br*1000)
			}
			frame = i
			_ = frame
			return info
		}
	}
	return Info{}
}

func parseMP3Header(b []byte) (bitrate, sampleRate, samplesPerFrame, frameLen int) {
	if len(b) < 4 || b[0] != 0xFF || b[1]&0xE0 != 0xE0 {
		return 0, 0, 0, 0
	}
	verBits := (b[1] >> 3) & 0x03 // 3=MPEG1, 2=MPEG2, 0=MPEG2.5
	layer := (b[1] >> 1) & 0x03   // 3=L1 2=L2 1=L3
	brIdx := int(b[2] >> 4)
	srIdx := int((b[2] >> 2) & 0x03)
	padding := int((b[2] >> 1) & 0x01)
	if verBits == 1 || layer == 0 || brIdx == 0 || brIdx == 15 || srIdx == 3 {
		return 0, 0, 0, 0
	}
	var rates = [3][3]int{
		{44100, 48000, 32000},
		{22050, 24000, 16000},
		{11025, 12000, 8000},
	}
	var vrow int
	switch verBits {
	case 3:
		vrow = 0
	case 2:
		vrow = 1
	default:
		vrow = 2
	}
	sampleRate = rates[vrow][srIdx]

	if layer == 1 { // Layer III
		if verBits == 3 {
			bitrate = mp3Bitrates[0][brIdx]
			samplesPerFrame = 1152
		} else {
			bitrate = mp3Bitrates[1][brIdx]
			samplesPerFrame = 576
		}
		frameLen = samplesPerFrame/8*bitrate*1000/sampleRate + padding
	} else if layer == 2 { // Layer II
		bitrate = mp3Bitrates[0][brIdx]
		samplesPerFrame = 1152
		frameLen = 144*bitrate*1000/sampleRate + padding
	} else { // Layer I
		bitrate = 32 * brIdx
		samplesPerFrame = 384
		frameLen = (12*bitrate*1000/sampleRate + padding) * 4
	}
	return
}

func xingFrames(b []byte) int {
	// Xing/Info lives after the side-info block.
	offsets := []int{36, 21, 13}
	for _, off := range offsets {
		if len(b) < off+12 {
			continue
		}
		tag := string(b[off : off+4])
		if tag != "Xing" && tag != "Info" {
			continue
		}
		flags := binary.BigEndian.Uint32(b[off+4 : off+8])
		if flags&0x1 == 0 {
			continue
		}
		return int(binary.BigEndian.Uint32(b[off+8 : off+12]))
	}
	if len(b) > 40 && string(b[36:40]) == "VBRI" {
		return int(binary.BigEndian.Uint32(b[36+14 : 36+18]))
	}
	return 0
}

// ---------------------------------------------------------------- flac

func probeFLAC(f *os.File, size int64) Info {
	head := make([]byte, 4+4+34)
	if _, err := f.ReadAt(head, 0); err != nil || string(head[0:4]) != "fLaC" {
		return Info{}
	}
	// First metadata block must be STREAMINFO.
	if head[4]&0x7f != 0 {
		return Info{}
	}
	si := head[8:]
	sr := int(si[10])<<12 | int(si[11])<<4 | int(si[12])>>4
	ch := int(si[12]>>1) & 0x07
	total := int64(si[13]&0x0f)<<32 | int64(si[14])<<24 | int64(si[15])<<16 | int64(si[16])<<8 | int64(si[17])
	info := Info{SampleRate: sr, Channels: ch + 1}
	if sr > 0 && total > 0 {
		info.Duration = float64(total) / float64(sr)
		info.Bitrate = int(float64(size) * 8 / info.Duration / 1000)
	}
	return info
}

// ---------------------------------------------------------------- ogg

func probeOgg(f *os.File, size int64) Info {
	head := make([]byte, 4096)
	n, _ := f.ReadAt(head, 0)
	head = head[:n]
	info := Info{}
	// Vorbis identification header: "\x01vorbis" ... sample rate LE at +12
	if i := bytes.Index(head, []byte("\x01vorbis")); i >= 0 && len(head) > i+16 {
		info.SampleRate = int(binary.LittleEndian.Uint32(head[i+12 : i+16]))
		info.Channels = int(head[i+11])
	} else if i := bytes.Index(head, []byte("OpusHead")); i >= 0 && len(head) > i+12 {
		info.SampleRate = 48000
		info.Channels = int(head[i+9])
	}
	rate := info.SampleRate
	if rate == 0 {
		rate = 44100
	}
	// Granule position of the final page = total PCM samples.
	tail := make([]byte, 128*1024)
	if size > int64(len(tail)) {
		f.ReadAt(tail, size-int64(len(tail)))
	} else {
		tail = tail[:size]
		f.ReadAt(tail, 0)
	}
	idx := bytes.LastIndex(tail, []byte("OggS"))
	if idx >= 0 && len(tail) >= idx+14 {
		g := int64(binary.LittleEndian.Uint64(tail[idx+6 : idx+14]))
		if g > 0 {
			info.Duration = float64(g) / float64(rate)
			if info.Duration > 0 {
				info.Bitrate = int(float64(size) * 8 / info.Duration / 1000)
			}
		}
	}
	return info
}

// ---------------------------------------------------------------- wav

func probeWav(f *os.File, size int64) Info {
	head := make([]byte, 12)
	if _, err := f.ReadAt(head, 0); err != nil {
		return Info{}
	}
	if string(head[0:4]) != "RIFF" || string(head[8:12]) != "WAVE" {
		return Info{}
	}
	info := Info{}
	byteRate := 0
	dataSize := int64(0)
	pos := int64(12)
	hdr := make([]byte, 8)
	for pos+8 < size {
		if _, err := f.ReadAt(hdr, pos); err != nil {
			break
		}
		id := string(hdr[0:4])
		sz := int64(binary.LittleEndian.Uint32(hdr[4:8]))
		switch id {
		case "fmt ":
			body := make([]byte, 16)
			if _, err := f.ReadAt(body, pos+8); err == nil {
				info.Channels = int(binary.LittleEndian.Uint16(body[2:4]))
				info.SampleRate = int(binary.LittleEndian.Uint32(body[4:8]))
				byteRate = int(binary.LittleEndian.Uint32(body[8:12]))
			}
		case "data":
			dataSize = sz
			if dataSize == 0 || pos+8+dataSize > size {
				dataSize = size - pos - 8
			}
		}
		if sz < 0 {
			break
		}
		pos += 8 + sz + (sz & 1)
	}
	if byteRate > 0 && dataSize > 0 {
		info.Duration = float64(dataSize) / float64(byteRate)
		info.Bitrate = int(float64(byteRate) * 8 / 1000)
	}
	return info
}

// ---------------------------------------------------------------- mp4 / m4a

func probeMP4(f *os.File, size int64) Info {
	info := Info{}
	var walk func(off, end int64, depth int) bool
	walk = func(off, end int64, depth int) bool {
		if depth > 4 {
			return false
		}
		hdr := make([]byte, 16)
		for off+8 <= end {
			if _, err := f.ReadAt(hdr[:8], off); err != nil {
				return false
			}
			boxSize := int64(binary.BigEndian.Uint32(hdr[0:4]))
			typ := string(hdr[4:8])
			headerLen := int64(8)
			if boxSize == 1 {
				if _, err := f.ReadAt(hdr[8:16], off+8); err != nil {
					return false
				}
				boxSize = int64(binary.BigEndian.Uint64(hdr[8:16]))
				headerLen = 16
			} else if boxSize == 0 {
				boxSize = end - off
			}
			if boxSize < headerLen {
				return false
			}
			if typ == "mvhd" {
				body := make([]byte, 40)
				if _, err := f.ReadAt(body, off+headerLen); err == nil {
					ver := body[0]
					if ver == 1 {
						timescale := binary.BigEndian.Uint32(body[20:24])
						dur := binary.BigEndian.Uint64(body[24:32])
						if timescale > 0 {
							info.Duration = float64(dur) / float64(timescale)
						}
					} else {
						timescale := binary.BigEndian.Uint32(body[12:16])
						dur := binary.BigEndian.Uint32(body[16:20])
						if timescale > 0 {
							info.Duration = float64(dur) / float64(timescale)
						}
					}
					return true
				}
			}
			if typ == "moov" || typ == "trak" || typ == "mdia" {
				if walk(off+headerLen, off+boxSize, depth+1) {
					return true
				}
			}
			off += boxSize
		}
		return false
	}
	walk(0, size, 0)
	if info.Duration > 0 {
		info.Bitrate = int(float64(size) * 8 / info.Duration / 1000)
	}
	return info
}

// ---------------------------------------------------------------- aiff / aifc

func ext80(b []byte) float64 {
	if len(b) < 10 {
		return 0
	}
	sign := 1.0
	if b[0]&0x80 != 0 {
		sign = -1
	}
	exp := int(b[0]&0x7f)<<8 | int(b[1])
	var mant uint64
	for _, v := range b[2:10] {
		mant = mant<<8 | uint64(v)
	}
	if exp == 0 && mant == 0 {
		return 0
	}
	return sign * math.Ldexp(float64(mant), exp-16383-63)
}

func probeAIFF(f *os.File, size int64) Info {
	head := make([]byte, 12)
	if _, err := f.ReadAt(head, 0); err != nil || string(head[0:4]) != "FORM" {
		return Info{}
	}
	if kind := string(head[8:12]); kind != "AIFF" && kind != "AIFC" {
		return Info{}
	}
	// Metadata chunks live at the front of the file; 1 MiB is plenty.
	buf := make([]byte, 1<<20)
	n, _ := f.ReadAt(buf, 12)
	buf = buf[:n]
	for pos := 0; pos+8 <= len(buf); {
		id := string(buf[pos : pos+4])
		sz := int(binary.BigEndian.Uint32(buf[pos+4 : pos+8]))
		if sz < 0 {
			break
		}
		if id == "COMM" {
			body := buf[pos+8:]
			if len(body) < 18 {
				break
			}
			ch := int(binary.BigEndian.Uint16(body[0:2]))
			frames := int64(binary.BigEndian.Uint32(body[2:6]))
			rate := ext80(body[8:18])
			info := Info{SampleRate: int(rate), Channels: ch}
			if rate > 0 && frames > 0 {
				info.Duration = float64(frames) / rate
			}
			return info
		}
		pos += 8 + sz + (sz & 1)
	}
	return Info{}
}

// ---------------------------------------------------------------- adts aac

func probeAAC(f *os.File, size int64) Info {
	br := bufio.NewReaderSize(f, 64*1024)
	if head, err := br.Peek(10); err == nil && string(head[:3]) == "ID3" {
		skip := int64(10)
		if head[5]&0x10 != 0 {
			skip += 10
		}
		tagSize := int64(head[6]&0x7f)<<21 | int64(head[7]&0x7f)<<14 |
			int64(head[8]&0x7f)<<7 | int64(head[9]&0x7f)
		_, _ = br.Discard(int(skip + tagSize))
	}
	rates := [...]int{96000, 88200, 64000, 48000, 44100, 32000, 24000, 22050, 16000, 12000, 11025, 8000, 7350}
	var info Info
	var samples int64
	for {
		h, err := br.Peek(7)
		if err != nil {
			break
		}
		if h[0] != 0xFF || h[1]&0xF6 != 0xF0 {
			_, _ = br.Discard(1)
			continue
		}
		srIdx := int(h[2]>>2) & 0x0f
		if srIdx == 15 {
			_, _ = br.Discard(1)
			continue
		}
		frameLen := int(h[3]&0x03)<<11 | int(h[4])<<3 | int(h[5])>>5
		headerLen := 7
		if h[1]&0x01 == 0 {
			headerLen = 9 // CRC present
		}
		if frameLen < headerLen {
			_, _ = br.Discard(1)
			continue
		}
		if info.SampleRate == 0 {
			info.SampleRate = rates[srIdx]
			info.Channels = int(h[2]&0x01)<<2 | int(h[3]>>6)&0x03
		}
		samples += int64(int(h[6]&0x03)+1) * 1024
		if _, err := br.Discard(frameLen); err != nil {
			break
		}
	}
	if info.SampleRate > 0 && samples > 0 {
		info.Duration = float64(samples) / float64(info.SampleRate)
		info.Bitrate = int(float64(size) * 8 / info.Duration / 1000)
	}
	return info
}

// ---------------------------------------------------------------- wma / asf

var (
	asfHeaderGUID    = []byte{0x30, 0x26, 0xB2, 0x75, 0x8E, 0x66, 0xCF, 0x11, 0xA6, 0xD9, 0x00, 0xAA, 0x00, 0x62, 0xCE, 0x6C}
	asfFilePropsGUID = []byte{0xA1, 0xDC, 0xAB, 0x8C, 0x47, 0xA9, 0xCF, 0x11, 0x8E, 0xE4, 0x00, 0xC0, 0x0C, 0x20, 0x53, 0x65}
)

func probeWMA(f *os.File, size int64) Info {
	head := make([]byte, 30)
	if _, err := f.ReadAt(head, 0); err != nil || !bytes.Equal(head[:16], asfHeaderGUID) {
		return Info{}
	}
	hsize := int64(binary.LittleEndian.Uint64(head[16:24]))
	if hsize < 30 {
		return Info{}
	}
	if hsize > size {
		hsize = size
	}
	if hsize > 4<<20 {
		hsize = 4 << 20
	}
	buf := make([]byte, hsize)
	n, _ := f.ReadAt(buf, 0)
	buf = buf[:n]
	for pos := int64(30); pos+24 <= int64(len(buf)); {
		if !bytes.Equal(buf[pos:pos+16], asfFilePropsGUID) {
			osz := int64(binary.LittleEndian.Uint64(buf[pos+16 : pos+24]))
			if osz < 24 {
				break
			}
			pos += osz
			continue
		}
		d := buf[pos+24:]
		if len(d) < 80 {
			break
		}
		playDur := int64(binary.LittleEndian.Uint64(d[40:48])) // 100 ns units
		preroll := int64(binary.LittleEndian.Uint64(d[56:64])) // milliseconds
		info := Info{}
		if playDur > 0 {
			seconds := float64(playDur)/1e7 - float64(preroll)/1000
			if seconds > 0 {
				info.Duration = seconds
			}
		}
		return info
	}
	return Info{}
}

// CopySeeker is a small helper for callers that need a reader over a file.
func CopySeeker(dst io.Writer, src io.ReadSeeker, offset int64) error {
	if _, err := src.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	_, err := io.Copy(dst, src)
	return err
}
