# Gera rsrc_windows_amd64.syso (COFF com secao .rsrc) contendo:
# icone do app, manifesto (pede administrador, estilos visuais, DPI) e informacoes de versao.
import io, struct, sys
from PIL import Image, ImageDraw

OUT = sys.argv[1] if len(sys.argv) > 1 else "rsrc_windows_amd64.syso"
VERSION = (2, 0, 0, 0)
LANG = 0x0409

# ---------- Icone ----------
def draw_icon(size=1024):
    im = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    s = size / 256
    d.rounded_rectangle([8*s, 8*s, 248*s, 248*s], radius=48*s, fill=(24, 28, 36, 255))
    acc = (255, 170, 40, 255)
    c = 128*s
    r = 70*s
    w = int(14*s)
    d.ellipse([c-r, c-r, c+r, c+r], outline=acc, width=w)
    gap, ln = 30*s, 100*s
    t = 7*s
    d.rectangle([c-t, c-ln, c+t, c-gap], fill=acc)
    d.rectangle([c-t, c+gap, c+t, c+ln], fill=acc)
    d.rectangle([c-ln, c-t, c-gap, c+t], fill=acc)
    d.rectangle([c+gap, c-t, c+ln, c+t], fill=acc)
    dot = 11*s
    d.ellipse([c-dot, c-dot, c+dot, c+dot], fill=(255, 255, 255, 255))
    return im

big = draw_icon()
# big.resize((512, 512), Image.LANCZOS).save("icon-preview.png")  # previa do icone
SIZES = [16, 24, 32, 48, 64, 256]
icons = []
for sz in SIZES:
    buf = io.BytesIO()
    big.resize((sz, sz), Image.LANCZOS).save(buf, "PNG")
    icons.append((sz, buf.getvalue()))

# ---------- Manifesto ----------
MANIFEST = b"""<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <assemblyIdentity type="win32" name="CS2Optimizer" version="2.0.0.0" processorArchitecture="amd64"/>
  <description>CS2 Optimizer</description>
  <dependency>
    <dependentAssembly>
      <assemblyIdentity type="win32" name="Microsoft.Windows.Common-Controls" version="6.0.0.0"
        processorArchitecture="*" publicKeyToken="6595b64144ccf1df" language="*"/>
    </dependentAssembly>
  </dependency>
  <trustInfo xmlns="urn:schemas-microsoft-com:asm.v3">
    <security>
      <requestedPrivileges>
        <requestedExecutionLevel level="requireAdministrator" uiAccess="false"/>
      </requestedPrivileges>
    </security>
  </trustInfo>
  <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
    <application>
      <supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/>
    </application>
  </compatibility>
  <application xmlns="urn:schemas-microsoft-com:asm.v3">
    <windowsSettings>
      <dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">true</dpiAware>
    </windowsSettings>
  </application>
</assembly>
"""

# ---------- Versao (VS_VERSIONINFO) ----------
def pad4(b):
    return b + b"\0" * (-len(b) % 4)

def u16z(s):
    return s.encode("utf-16-le") + b"\0\0"

def node(key, value=b"", wtype=1, children=(), value_len=None):
    # bytes contados a partir do inicio da estrutura, para alinhar em 32 bits corretamente
    b = pad4(b"\0" * 6 + u16z(key)) + value
    if children:
        b = pad4(b)
        for k in children[:-1]:
            b += pad4(k)
        b += children[-1]
    if value_len is None:
        value_len = len(value) // 2 if wtype == 1 else len(value)
    return struct.pack("<HHH", len(b), value_len, wtype) + b[6:]

ms = (VERSION[0] << 16) | VERSION[1]
ls = (VERSION[2] << 16) | VERSION[3]
fixed = struct.pack("<13I", 0xFEEF04BD, 0x00010000, ms, ls, ms, ls, 0x3F, 0, 0x00040004, 1, 0, 0, 0)
vstr = "%d.%d.%d.%d" % VERSION
strings = [
    ("CompanyName", "CS2 Optimizer"),
    ("FileDescription", "CS2 Optimizer - otimizacao segura do Windows para CS2"),
    ("FileVersion", vstr),
    ("InternalName", "CS2Optimizer"),
    ("OriginalFilename", "CS2 Optimizer.exe"),
    ("ProductName", "CS2 Optimizer"),
    ("ProductVersion", vstr),
]
str_nodes = [node(k, u16z(v), 1) for k, v in strings]
table = node("041604B0", b"", 1, str_nodes, 0)
sfi = node("StringFileInfo", b"", 1, [table], 0)
var = node("Translation", struct.pack("<HH", 0x0416, 0x04B0), 0)
vfi = node("VarFileInfo", b"", 1, [var], 0)
versioninfo = node("VS_VERSION_INFO", fixed, 0, [sfi, vfi])

# ---------- Grupo de icones ----------
grp = struct.pack("<HHH", 0, 1, len(icons))
for i, (sz, png) in enumerate(icons):
    b = 0 if sz >= 256 else sz
    grp += struct.pack("<BBBBHHIH", b, b, 0, 0, 1, 32, len(png), i + 1)

# ---------- Arvore de recursos ----------
RT_ICON, RT_GROUP_ICON, RT_VERSION, RT_MANIFEST = 3, 14, 16, 24
resources = {
    RT_ICON: {i + 1: png for i, (sz, png) in enumerate(icons)},
    RT_GROUP_ICON: {1: grp},
    RT_VERSION: {1: versioninfo},
    RT_MANIFEST: {1: MANIFEST},
}

types = sorted(resources)
n_types = len(types)
n_names = sum(len(resources[t]) for t in types)
DIR_HDR, ENT = 16, 8
root_size = DIR_HDR + ENT * n_types
type_dirs_size = sum(DIR_HDR + ENT * len(resources[t]) for t in types)
lang_dirs_size = n_names * (DIR_HDR + ENT)
data_entries_off = root_size + type_dirs_size + lang_dirs_size
data_start = data_entries_off + 16 * n_names
data_start += -data_start % 8

def dirhdr(n):
    return struct.pack("<IIHHHH", 0, 0, 0, 0, 0, n)

sec = bytearray()
relocs = []
# raiz
sec += dirhdr(n_types)
off = root_size
type_offs = []
for t in types:
    type_offs.append(off)
    off += DIR_HDR + ENT * len(resources[t])
for t, o in zip(types, type_offs):
    sec += struct.pack("<II", t, 0x80000000 | o)
# diretorios de nome
lang_off = root_size + type_dirs_size
lang_list = []
for t in types:
    ids = sorted(resources[t])
    sec += dirhdr(len(ids))
    for rid in ids:
        sec += struct.pack("<II", rid, 0x80000000 | lang_off)
        lang_list.append((t, rid))
        lang_off += DIR_HDR + ENT
# diretorios de idioma
de_off = data_entries_off
for _ in lang_list:
    sec += dirhdr(1)
    sec += struct.pack("<II", LANG, de_off)  # sem bit alto -> data entry
    de_off += 16
assert len(sec) == data_entries_off
# data entries + dados
blobs = []
cur = data_start
for (t, rid) in lang_list:
    blob = resources[t][rid]
    relocs.append(len(sec))
    sec += struct.pack("<IIII", cur, len(blob), 0, 0)
    blobs.append(blob)
    cur += len(blob)
    cur += -cur % 8
sec += b"\0" * (data_start - len(sec))
for blob in blobs:
    sec += blob
    sec += b"\0" * (-len(sec) % 8)

# ---------- COFF ----------
FILE_HDR, SEC_HDR = 20, 40
raw_ptr = FILE_HDR + SEC_HDR
reloc_ptr = raw_ptr + len(sec)
sym_ptr = reloc_ptr + 10 * len(relocs)
out = bytearray()
out += struct.pack("<HHIIIHH", 0x8664, 1, 0, sym_ptr, 1, 0, 0)
out += struct.pack("<8sIIIIIIHHI", b".rsrc\0\0\0", 0, 0, len(sec), raw_ptr, reloc_ptr, 0,
                   len(relocs), 0, 0x40000040)
out += sec
for r in relocs:
    out += struct.pack("<IIH", r, 0, 3)  # IMAGE_REL_AMD64_ADDR32NB -> simbolo .rsrc
out += struct.pack("<8sIhHBB", b".rsrc\0\0\0", 0, 1, 0, 3, 0)
out += struct.pack("<I", 4)  # tabela de strings vazia
open(OUT, "wb").write(out)
print("ok", OUT, len(out), "bytes;", len(relocs), "recursos")
