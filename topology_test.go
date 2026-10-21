package main

import (
	"encoding/binary"
	"testing"
)

func coreEntry(class uint8, mask uint64) []byte {
	b := make([]byte, 48)
	binary.LittleEndian.PutUint32(b[0:], 0)  // RelationProcessorCore
	binary.LittleEndian.PutUint32(b[4:], 48) // Size
	b[9] = class
	binary.LittleEndian.PutUint16(b[30:], 1) // GroupCount
	binary.LittleEndian.PutUint64(b[32:], mask)
	return b
}

// i7-13700K: 8 nucleos P com HT (16 logicos) + 8 nucleos E (logicos 16..23)
func TestTopology13700K(t *testing.T) {
	var buf []byte
	for i := 0; i < 8; i++ {
		buf = append(buf, coreEntry(1, 3<<(2*i))...)
	}
	for i := 0; i < 8; i++ {
		buf = append(buf, coreEntry(0, 1<<(16+i))...)
	}
	topo := parseTopology(buf)
	if !topo.hybrid || topo.pCores != 8 || topo.eCores != 8 || topo.pMask != 0xFFFF || topo.eMask != 0xFF0000 {
		t.Fatalf("%+v", topo)
	}
}

func TestTopologyNonHybrid(t *testing.T) {
	var buf []byte
	for i := 0; i < 6; i++ {
		buf = append(buf, coreEntry(0, 3<<(2*i))...)
	}
	if parseTopology(buf).hybrid {
		t.Fatal("6 nucleos iguais nao e hibrido")
	}
}

func TestMicrocodeRegex(t *testing.T) {
	re := raptorDesktopRe
	yes := []string{"13th Gen Intel(R) Core(TM) i9-13900K", "14th Gen Intel(R) Core(TM) i7-14700KF", "13th Gen Intel(R) Core(TM) i5-13400F", "14th Gen Intel(R) Core(TM) i9-14900KS", "14th Gen Intel(R) Core(TM) i5-14400"}
	no := []string{"13th Gen Intel(R) Core(TM) i7-13700H", "13th Gen Intel(R) Core(TM) i9-13980HX", "12th Gen Intel(R) Core(TM) i7-12700K", "AMD Ryzen 7 7800X3D 8-Core Processor"}
	for _, s := range yes {
		if !re.MatchString(s) {
			t.Error("deveria detectar:", s)
		}
	}
	for _, s := range no {
		if re.MatchString(s) {
			t.Error("nao deveria detectar:", s)
		}
	}
}
