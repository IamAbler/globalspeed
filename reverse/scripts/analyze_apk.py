"""Reproducible, offline APK inventory and configuration extraction."""
import hashlib, json, pathlib, re, struct, zipfile
import xml.etree.ElementTree as ET

ROOT = pathlib.Path(__file__).resolve().parents[2]
apk = ROOT / 'reverse/samples/globalspeed_4.4.8_safe.apk'
out = ROOT / 'reverse/analysis'
out.mkdir(exist_ok=True)
z = zipfile.ZipFile(apk)
xml = z.read('assets/settings_v3.xml')
(out / 'settings_v3.xml').write_bytes(xml)
modules = []
for m in ET.fromstring(xml).find('module_list'):
    modules.append({**m.attrib, 'items': [{**i.attrib, 'target': i.text.strip()} for i in m]})
(ROOT / 'internal/catalog/tests.json').write_text(json.dumps(modules, ensure_ascii=False, indent=2))
native = z.read('lib/arm64-v8a/libGSCore.so')
strings = [v.decode() for v in re.findall(rb'[\x20-\x7e]{4,}', native)]
(out / 'native-strings.txt').write_text('\n'.join(strings))
# DEX string table; packed payloads are not claimed to be recovered.
dex = z.read('classes.dex')
count, offset = struct.unpack_from('<II', dex, 56)
dex_strings = []
for i in range(count):
    p = struct.unpack_from('<I', dex, offset + i * 4)[0]
    while dex[p] & 128: p += 1
    p += 1
    end = dex.find(b'\0', p)
    dex_strings.append(dex[p:end].decode('utf-8', errors='replace'))
(out / 'dex-strings.txt').write_text('\n'.join(dex_strings))
# Decode Android binary XML string pool and element attributes.
data = z.read('AndroidManifest.xml')
pool, elements = [], []
p = 8
def length(pos, wide=False):
    if wide:
        v = struct.unpack_from('<H', data, pos)[0]; pos += 2
        if v & 0x8000: v = ((v & 0x7fff) << 16) | struct.unpack_from('<H', data, pos)[0]; pos += 2
    else:
        v = data[pos]; pos += 1
        if v & 128: v = ((v & 127) << 8) | data[pos]; pos += 1
    return v, pos
while p < len(data):
    typ, header, size = struct.unpack_from('<HHI', data, p)
    if size < 8: raise ValueError('Invalid AXML chunk')
    if typ == 1:
        n, _, flags, start, _ = struct.unpack_from('<IIIII', data, p + 8)
        for i in range(n):
            q = p + start + struct.unpack_from('<I', data, p + header + i * 4)[0]
            if flags & 256:
                _, q = length(q); size_s, q = length(q)
                pool.append(data[q:q+size_s].decode('utf-8', errors='replace'))
            else:
                size_s, q = length(q, True)
                pool.append(data[q:q+size_s*2].decode('utf-16le', errors='replace'))
    elif typ == 0x102:
        name = pool[struct.unpack_from('<I', data, p+20)[0]]
        astart, asize, acount = struct.unpack_from('<HHH', data, p+24)
        attrs = {}
        for i in range(acount):
            q = p + 16 + astart + i * asize
            _, ni, raw, _, _, vt, value = struct.unpack_from('<IIIHBBI', data, q)
            attrs[pool[ni]] = pool[raw] if raw != 0xffffffff else (pool[value] if vt == 3 else value)
        elements.append({'element': name, 'attributes': attrs})
    p += size
(out / 'manifest.json').write_text(json.dumps(elements, ensure_ascii=False, indent=2))
inventory = {'apk': apk.name, 'sha256': hashlib.sha256(apk.read_bytes()).hexdigest(),
    'files': [{'name': f.filename, 'size': f.file_size} for f in z.infolist()],
    'dex_strings': count, 'modules': len(modules), 'targets': sum(len(m['items']) for m in modules),
    'packing_evidence': [n for n in z.namelist() if 'jiagu' in n or n == 'assets/.jgapp']}
(out / 'inventory.json').write_text(json.dumps(inventory, ensure_ascii=False, indent=2))
print(json.dumps({k:v for k,v in inventory.items() if k != 'files'}, ensure_ascii=False))
