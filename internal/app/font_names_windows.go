//go:build windows

package app

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf16"

	"golang.org/x/sys/windows/registry"
)

// localizedFontNames maps each installed font family's English name to its
// Chinese one, as the font's own name table holds it: the name Windows'
// font folder shows it under. The fonts come through the registry once.
var localizedFontNames = sync.OnceValue(func() map[string]string {
	names := map[string]string{}
	add := func(root registry.Key) {
		k, err := registry.OpenKey(root, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts`, registry.QUERY_VALUE)
		if err != nil {
			return
		}
		defer k.Close()
		entries, err := k.ReadValueNames(-1)
		if err != nil {
			return
		}
		for _, entry := range entries {
			file, _, err := k.GetStringValue(entry)
			if err != nil || !fontFile(file) {
				continue
			}
			if !filepath.IsAbs(file) {
				file = filepath.Join(os.Getenv("WINDIR"), "Fonts", file)
			}
			f, err := os.Open(file)
			if err != nil {
				continue
			}
			for _, n := range sfntFamilyNames(f) {
				if _, seen := names[n.en]; !seen {
					names[n.en] = n.zh
				}
			}
			f.Close()
		}
	}
	add(registry.LOCAL_MACHINE)
	add(registry.CURRENT_USER)
	return names
})

// fontFile reports whether a registry entry points at a file the names are
// read from.
func fontFile(file string) bool {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".ttf", ".otf", ".ttc", ".otc":
		return true
	}
	return false
}

// familyName is a font's family name in the two languages the lists want.
type familyName struct{ en, zh string }

// sfntFamilyNames reads the family names of each font in a font file: one,
// or a collection of them.
func sfntFamilyNames(f *os.File) []familyName {
	var tag [4]byte
	if _, err := f.ReadAt(tag[:], 0); err != nil {
		return nil
	}
	if string(tag[:]) != "ttcf" {
		if en, zh, ok := fontAt(f, 0); ok {
			return []familyName{{en, zh}}
		}
		return nil
	}
	var hdr [4]byte
	if _, err := f.ReadAt(hdr[:], 8); err != nil { // past the tag and version
		return nil
	}
	var out []familyName
	for i := range binary.BigEndian.Uint32(hdr[:]) {
		var off [4]byte
		if _, err := f.ReadAt(off[:], 12+4*int64(i)); err != nil {
			break
		}
		if en, zh, ok := fontAt(f, int64(binary.BigEndian.Uint32(off[:]))); ok {
			out = append(out, familyName{en, zh})
		}
	}
	return out
}

// fontAt reads the names of the font at a byte offset in a file.
func fontAt(f *os.File, at int64) (en, zh string, ok bool) {
	var head [12]byte
	if _, err := f.ReadAt(head[:], at); err != nil {
		return "", "", false
	}
	records := make([]byte, 16*int(binary.BigEndian.Uint16(head[4:6])))
	if _, err := f.ReadAt(records, at+12); err != nil {
		return "", "", false
	}
	for i := 0; i+16 <= len(records); i += 16 {
		rec := records[i : i+16 : i+16]
		if string(rec[:4]) != "name" {
			continue
		}
		table := make([]byte, binary.BigEndian.Uint32(rec[12:16]))
		if _, err := f.ReadAt(table, at+int64(binary.BigEndian.Uint32(rec[8:12]))); err != nil {
			return "", "", false
		}
		return nameTableFamilyNames(table)
	}
	return "", "", false
}

// nameTableFamilyNames reads the English and Chinese family names out of a
// font's name table: its typographic family, or the family where the font
// keeps no separate one.
func nameTableFamilyNames(table []byte) (en, zh string, ok bool) {
	if len(table) < 6 {
		return "", "", false
	}
	count := int(binary.BigEndian.Uint16(table[2:4]))
	strs := int(binary.BigEndian.Uint16(table[4:6]))
	en16, en1, zh16, zh1 := "", "", "", ""
	for i := 0; i < count && 6+12*(i+1) <= len(table); i++ {
		rec := table[6+12*i : 6+12*i+12]
		if binary.BigEndian.Uint16(rec[0:2]) != 3 || // Windows
			binary.BigEndian.Uint16(rec[2:4]) != 1 { // Unicode, as Go reads it
			continue
		}
		lang := binary.BigEndian.Uint16(rec[4:6])
		name := binary.BigEndian.Uint16(rec[6:8])
		if name != 16 && name != 1 {
			continue
		}
		from := strs + int(binary.BigEndian.Uint16(rec[10:12]))
		to := from + int(binary.BigEndian.Uint16(rec[8:10]))
		if from > to || to > len(table) {
			continue
		}
		value := decodeUTF16BE(table[from:to])
		switch {
		case lang == 0x409 && name == 16: // English (United States)
			en16 = value
		case lang == 0x409:
			en1 = value
		case lang == 0x804 && name == 16: // Chinese (Simplified)
			zh16 = value
		case lang == 0x804:
			zh1 = value
		}
	}
	en, zh = en16, zh16
	if en == "" {
		en = en1
	}
	if zh == "" {
		zh = zh1
	}
	return en, zh, en != "" && zh != ""
}

// decodeUTF16BE reads a name table string, which holds UTF-16 in the big
// end the table's own numbers are in.
func decodeUTF16BE(b []byte) string {
	units := make([]uint16, len(b)/2)
	for i := range units {
		units[i] = binary.BigEndian.Uint16(b[2*i:])
	}
	return string(utf16.Decode(units))
}
