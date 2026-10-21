package main

import (
	"encoding/binary"
	"regexp"
)

// Intel 13a/14a geracao de desktop (os afetados pela instabilidade "Vmin Shift")
var raptorDesktopRe = regexp.MustCompile(`(?i)(13th|14th) Gen Intel.*i[3579]-1[34]\d{3}([KF]|KF|KS|T)?\s*$`)

type cpuTopology struct {
	hybrid       bool
	pMask, eMask uint64
	pCores       int // nucleos fisicos P
	eCores       int // nucleos fisicos E
}

// parseTopology le o buffer de GetLogicalProcessorInformationEx(RelationProcessorCore).
// Layout de cada item: Relationship u32, Size u32, Flags u8, EfficiencyClass u8, Reserved[20],
// GroupCount u16 (offset 30), GroupMask[] { Mask u64 (offset 32), Group u16 (offset 40), ... }.
func parseTopology(buf []byte) cpuTopology {
	var t cpuTopology
	type core struct {
		class uint8
		mask  uint64
	}
	var cores []core
	maxClass := uint8(0)
	size := uint32(len(buf))
	for off := uint32(0); off+48 <= size; {
		rel := binary.LittleEndian.Uint32(buf[off:])
		sz := binary.LittleEndian.Uint32(buf[off+4:])
		if sz == 0 {
			break
		}
		if rel == 0 { // RelationProcessorCore
			class := buf[off+9]
			if binary.LittleEndian.Uint16(buf[off+30:]) >= 1 {
				if binary.LittleEndian.Uint16(buf[off+40:]) != 0 {
					return cpuTopology{} // mais de 64 processadores logicos: nao mexe
				}
				mask := binary.LittleEndian.Uint64(buf[off+32:])
				cores = append(cores, core{class, mask})
				if class > maxClass {
					maxClass = class
				}
			}
		}
		off += sz
	}
	for _, c := range cores {
		if c.class == maxClass {
			t.pMask |= c.mask
			t.pCores++
		} else {
			t.eMask |= c.mask
			t.eCores++
		}
	}
	t.hybrid = t.pCores > 0 && t.eCores > 0
	return t
}
