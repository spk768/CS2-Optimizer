package main

// Programas que abrem com o Windows. Liga/desliga do mesmo jeito que o Gerenciador de Tarefas
// (chaves StartupApproved), sem apagar nada, e o "Restaurar tudo" desfaz.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unsafe"
)

var pRegEnumValueW = advapi32.NewProc("RegEnumValueW")

const approvedBase = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\`

type startupEntry struct {
	name, command string
	root          string // HKCU ou HKLM (onde fica a chave StartupApproved)
	approvedKey   string
	valueName     string
	enabled       bool
}

// regValueNames lista os valores (nome e texto) de uma chave
func regValueNames(root uintptr, path string) map[string]string {
	out := map[string]string{}
	var h syscall.Handle
	if r, _, _ := pRegOpenKeyExW.Call(root, uintptr(unsafe.Pointer(u16(path))), 0, KEY_READ, uintptr(unsafe.Pointer(&h))); r != 0 {
		return out
	}
	defer pRegCloseKey.Call(uintptr(h))
	for i := uint32(0); i < 512; i++ {
		var name [512]uint16
		nameLen := uint32(len(name))
		var typ uint32
		data := make([]byte, 4096)
		dataLen := uint32(len(data))
		r, _, _ := pRegEnumValueW.Call(uintptr(h), uintptr(i), uintptr(unsafe.Pointer(&name[0])), uintptr(unsafe.Pointer(&nameLen)), 0,
			uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(&dataLen)))
		if r == 259 { // ERROR_NO_MORE_ITEMS
			break
		}
		if r != 0 {
			continue
		}
		n := syscall.UTF16ToString(name[:nameLen])
		if n == "" {
			continue
		}
		val := ""
		if typ == REG_SZ || typ == 2 { // REG_SZ / REG_EXPAND_SZ
			val = regString(regValue{true, typ, data[:dataLen]})
		}
		out[n] = val
	}
	return out
}

func approvedEnabled(root uintptr, key, name string) bool {
	v := regRead(root, approvedBase+key, name)
	if !v.Exists || len(v.Data) == 0 {
		return true
	}
	return v.Data[0]&1 == 0 // 02/06 = ligado, 03/07 = desligado
}

func listStartup() []startupEntry {
	var list []startupEntry
	add := func(root uintptr, rootName, runKey, approvedKey string) {
		for name, cmd := range regValueNames(root, runKey) {
			list = append(list, startupEntry{name, cmd, rootName, approvedKey, name, approvedEnabled(root, approvedKey, name)})
		}
	}
	add(HKCU, "HKCU", `Software\Microsoft\Windows\CurrentVersion\Run`, "Run")
	add(HKLM, "HKLM", `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, "Run")
	add(HKLM, "HKLM", `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Run`, "Run32")
	folders := []struct {
		dir, root string
		rootH     uintptr
	}{
		{filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs\Startup`), "HKCU", HKCU},
		{filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs\StartUp`), "HKLM", HKLM},
	}
	for _, f := range folders {
		entries, _ := os.ReadDir(f.dir)
		for _, e := range entries {
			if e.IsDir() || strings.EqualFold(e.Name(), "desktop.ini") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			list = append(list, startupEntry{name, filepath.Join(f.dir, e.Name()), f.root, "StartupFolder", e.Name(), approvedEnabled(f.rootH, "StartupFolder", e.Name())})
		}
	}
	sort.Slice(list, func(i, j int) bool { return strings.ToLower(list[i].name) < strings.ToLower(list[j].name) })
	return list
}

func setStartupEnabled(e startupEntry, on bool) error {
	data := make([]byte, 12)
	if on {
		data[0] = 2
	} else {
		data[0] = 3
	}
	return setReg(e.root, approvedBase+e.approvedKey, e.valueName, 3 /* REG_BINARY */, data)
}

// startupDesc encurta o comando para caber numa linha
func startupDesc(cmd string) string {
	c := strings.TrimSpace(cmd)
	if strings.HasPrefix(c, `"`) {
		if i := strings.Index(c[1:], `"`); i >= 0 {
			c = c[1 : i+1]
		}
	} else if i := strings.Index(strings.ToLower(c), ".exe"); i >= 0 {
		c = c[:i+4]
	}
	if c == "" {
		return "(sem caminho)"
	}
	return c
}
