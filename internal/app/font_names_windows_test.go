//go:build windows

package app

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// nameRecord is one entry of a name table the tests build: a family name
// in a language.
type nameRecord struct {
	lang uint16 // 0x409 English, 0x804 Chinese
	id   uint16 // 16 typographic family, 1 family
	text string
}

// buildNameTable lays out a font's name table holding the records given.
func buildNameTable(records []nameRecord) []byte {
	strs := &bytes.Buffer{}
	recs := &bytes.Buffer{}
	for _, r := range records {
		var text bytes.Buffer
		for _, unit := range []rune(r.text) {
			var b [2]byte
			binary.BigEndian.PutUint16(b[:], uint16(unit))
			text.Write(b[:])
		}
		recs.Write([]byte{0, 3, 0, 1}) // a Windows name in Unicode
		var f [2]byte
		binary.BigEndian.PutUint16(f[:], r.lang)
		recs.Write(f[:])
		binary.BigEndian.PutUint16(f[:], r.id)
		recs.Write(f[:])
		binary.BigEndian.PutUint16(f[:], uint16(text.Len()))
		recs.Write(f[:])
		binary.BigEndian.PutUint16(f[:], uint16(strs.Len()))
		recs.Write(f[:])
		strs.Write(text.Bytes())
	}
	table := &bytes.Buffer{}
	table.Write([]byte{0, 0})
	var c [2]byte
	binary.BigEndian.PutUint16(c[:], uint16(len(records)))
	table.Write(c[:])
	binary.BigEndian.PutUint16(c[:], uint16(6+12*len(records))) // the strings start after the records
	table.Write(c[:])
	table.Write(recs.Bytes())
	table.Write(strs.Bytes())
	return table.Bytes()
}

// buildFontFile lays out a font file whose single table holds the records.
func buildFontFile(records []nameRecord) []byte {
	table := buildNameTable(records)
	file := &bytes.Buffer{}
	file.WriteString("\x00\x01\x00\x00")
	file.Write([]byte{0, 1, 0, 16, 0, 0, 0, 0}) // one table
	file.WriteString("name")
	file.Write([]byte{0, 0, 0, 0}) // a checksum of no matter
	var o [4]byte
	binary.BigEndian.PutUint32(o[:], 28)
	file.Write(o[:])
	binary.BigEndian.PutUint32(o[:], uint32(len(table)))
	file.Write(o[:])
	file.Write(table)
	return file.Bytes()
}

// The name table's own names come back: the typographic family in English
// and in Chinese, or the family where the font keeps no separate one.
func TestNameTableFamilyNames(t *testing.T) {
	table := buildNameTable([]nameRecord{
		{lang: 0x409, id: 16, text: "WPS LingXiu Hei"},
		{lang: 0x804, id: 16, text: "WPS灵秀黑"},
	})
	en, zh, ok := nameTableFamilyNames(table)
	if !ok || en != "WPS LingXiu Hei" || zh != "WPS灵秀黑" {
		t.Fatalf("got %q / %q / %v", en, zh, ok)
	}

	table = buildNameTable([]nameRecord{
		{lang: 0x409, id: 1, text: "SimHei"},
		{lang: 0x804, id: 1, text: "黑体"},
	})
	if en, zh, ok = nameTableFamilyNames(table); !ok || en != "SimHei" || zh != "黑体" {
		t.Fatalf("fell back to the family: got %q / %q / %v", en, zh, ok)
	}

	// A font without a Chinese name of its own doesn't come through as one.
	table = buildNameTable([]nameRecord{{lang: 0x409, id: 16, text: "Arial"}})
	if _, _, ok = nameTableFamilyNames(table); ok {
		t.Fatal("counted a font with no Chinese name")
	}
}

// A font file read from disk gives the names its name table holds, and one
// holding several fonts gives each of theirs.
func TestSfntFamilyNames(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "one.ttf")
	if err := os.WriteFile(file, buildFontFile([]nameRecord{
		{lang: 0x409, id: 16, text: "WPS LingXiu Hei"},
		{lang: 0x804, id: 16, text: "WPS灵秀黑"},
	}), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	names := sfntFamilyNames(f)
	if len(names) != 1 || names[0].en != "WPS LingXiu Hei" || names[0].zh != "WPS灵秀黑" {
		t.Fatalf("got %+v", names)
	}

	// A collection: two fonts in one file, each with its own table.
	one := buildFontFile([]nameRecord{{lang: 0x409, id: 16, text: "SimSun"}, {lang: 0x804, id: 16, text: "宋体"}})
	two := buildFontFile([]nameRecord{{lang: 0x409, id: 16, text: "NSimSun"}, {lang: 0x804, id: 16, text: "新宋体"}})
	var ttc bytes.Buffer
	ttc.WriteString("ttcf")
	ttc.Write([]byte{0, 1, 0, 0, 0, 0, 0, 2}) // two fonts
	var off [4]byte
	binary.BigEndian.PutUint32(off[:], 20)
	ttc.Write(off[:])
	binary.BigEndian.PutUint32(off[:], uint32(20+len(one)))
	ttc.Write(off[:])
	ttc.Write(one)
	ttc.Write(two)
	file = filepath.Join(dir, "two.ttc")
	if err := os.WriteFile(file, ttc.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	f2, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()
	names = sfntFamilyNames(f2)
	if len(names) != 2 || names[0].zh != "宋体" || names[1].zh != "新宋体" {
		t.Fatalf("got %+v", names)
	}
}
