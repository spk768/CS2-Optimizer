package main

// Leitura e escrita no Registro do Windows.

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const (
	HKCU      = 0x80000001
	HKLM      = 0x80000002
	REG_SZ    = 1
	REG_DWORD = 4
	KEY_READ  = 0x20019
	KEY_WRITE = 0x20006
)

type regValue struct {
	Exists bool
	Type   uint32
	Data   []byte
}

func rootOf(name string) uintptr {
	if name == "HKLM" {
		return HKLM
	}
	return HKCU
}

func regRead(root uintptr, path, name string) regValue {
	var h syscall.Handle
	r, _, _ := pRegOpenKeyExW.Call(root, uintptr(unsafe.Pointer(u16(path))), 0, KEY_READ, uintptr(unsafe.Pointer(&h)))
	if r != 0 {
		return regValue{}
	}
	defer pRegCloseKey.Call(uintptr(h))
	n := u16(name)
	var typ, size uint32
	r, _, _ = pRegQueryValueExW.Call(uintptr(h), uintptr(unsafe.Pointer(n)), 0, uintptr(unsafe.Pointer(&typ)), 0, uintptr(unsafe.Pointer(&size)))
	if r != 0 {
		return regValue{}
	}
	buf := make([]byte, size+2)
	size = uint32(len(buf))
	r, _, _ = pRegQueryValueExW.Call(uintptr(h), uintptr(unsafe.Pointer(n)), 0, uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r != 0 {
		return regValue{}
	}
	return regValue{Exists: true, Type: typ, Data: buf[:size]}
}

func regWrite(root uintptr, path, name string, typ uint32, data []byte) error {
	var h syscall.Handle
	r, _, _ := pRegCreateKeyExW.Call(root, uintptr(unsafe.Pointer(u16(path))), 0, 0, 0, KEY_READ|KEY_WRITE, 0, uintptr(unsafe.Pointer(&h)), 0)
	if r != 0 {
		return fmt.Errorf("não foi possível abrir %s (erro %d)", path, r)
	}
	defer pRegCloseKey.Call(uintptr(h))
	var p uintptr
	if len(data) > 0 {
		p = uintptr(unsafe.Pointer(&data[0]))
	}
	r, _, _ = pRegSetValueExW.Call(uintptr(h), uintptr(unsafe.Pointer(u16(name))), 0, uintptr(typ), p, uintptr(len(data)))
	if r != 0 {
		return fmt.Errorf("não foi possível gravar %s (erro %d)", name, r)
	}
	return nil
}

func regDelete(root uintptr, path, name string) error {
	var h syscall.Handle
	r, _, _ := pRegOpenKeyExW.Call(root, uintptr(unsafe.Pointer(u16(path))), 0, KEY_READ|KEY_WRITE, uintptr(unsafe.Pointer(&h)))
	if r == 2 {
		return nil // chave nao existe: nada a apagar
	}
	if r != 0 {
		return fmt.Errorf("não foi possível abrir %s (erro %d)", path, r)
	}
	defer pRegCloseKey.Call(uintptr(h))
	r, _, _ = pRegDeleteValueW.Call(uintptr(h), uintptr(unsafe.Pointer(u16(name))))
	if r != 0 && r != 2 {
		return fmt.Errorf("não foi possível apagar %s (erro %d)", name, r)
	}
	return nil
}

func strData(s string) []byte {
	u := utf16.Encode([]rune(s))
	b := make([]byte, (len(u)+1)*2)
	for i, c := range u {
		binary.LittleEndian.PutUint16(b[i*2:], c)
	}
	return b
}

func dwordData(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}

func regString(v regValue) string {
	if !v.Exists || len(v.Data) < 2 {
		return ""
	}
	u := make([]uint16, len(v.Data)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(v.Data[i*2:])
	}
	return syscall.UTF16ToString(u)
}

func regDword(v regValue) (uint32, bool) {
	if !v.Exists || v.Type != REG_DWORD || len(v.Data) < 4 {
		return 0, false
	}
	return binary.LittleEndian.Uint32(v.Data), true
}
