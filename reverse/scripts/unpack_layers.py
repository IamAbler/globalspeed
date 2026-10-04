"""Sample-specific static recovery of Jiagu configuration and inner loader.

This does NOT yet decrypt the application's protected DEX blocks.
All constants below were identified in this APK's loader disassembly.
"""
import json, pathlib, struct, zipfile, zlib

ROOT = pathlib.Path(__file__).resolve().parents[2]
OUT = ROOT / 'reverse/analysis'
z = zipfile.ZipFile(ROOT / 'reverse/samples/globalspeed_4.4.8_safe.apk')
dex = z.read('classes.dex')
shell_end = sum(struct.unpack_from('<II', dex, 104))
payload = dex[shell_end:]
assert payload[:4] == bytes.fromhex('71680001')
assert struct.unpack_from('<I', payload, 4)[0] == len(payload) - 8
config_size = struct.unpack_from('<I', payload, 8)[0]
# Recovered from inner loader 0x5fac8: byte addition followed by XOR.
config = bytearray(((value + 0x70) & 255) ^ 0x36
                   for value in payload[12:12+config_size])
p = 0
settings = {}
while p < len(config):
    tag, key_size, value_size = struct.unpack_from('<III', config, p)
    assert tag == 0x6b70
    p += 12
    key = config[p:p+key_size].decode(); p += key_size
    value = config[p:p+value_size].decode(); p += value_size
    settings[key] = value
assert p == len(config) and int(settings['ls']) == shell_end
(OUT / 'packer-config.json').write_text(json.dumps(settings, ensure_ascii=False, indent=2))
p = 12 + config_size
count = struct.unpack_from('<I', payload, p)[0]; p += 4
blocks = []
for i in range(count):
    size = struct.unpack_from('<I', payload, p)[0]; p += 4
    data = payload[p:p+size]; assert len(data) == size
    prefix_size = struct.unpack_from('<I', data)[0]
    assert prefix_size + 4 <= size
    (OUT / f'business-block-{i+1}.bin').write_bytes(data)
    blocks.append({'index':i+1, 'apk_dex_offset':shell_end+p, 'size':size,
        'protected_prefix_size':prefix_size, 'remaining_size':size-prefix_size-4})
    p += size
assert p == len(payload)
(OUT / 'business-blocks.json').write_text(json.dumps(blocks, indent=2))

loader = z.read('assets/libjiagu_a64.so')
# 0xebbc..0xebe0 loads these ten bytes and calls KSA at 0xdcd4.
key = loader[0x4ec41:0x4ec49] + struct.pack('<H', 0x7456)
# KSA at 0xdcd4, modified PRGA at 0xddfc: i+=2, j+=S[i]+1;
# state starts at i=3,j=5. Verified against instructions in this sample.
S = list(range(256)); j = 0
for i in range(256):
    j = (j + S[i] + key[i % len(key)]) & 255
    S[i], S[j] = S[j], S[i]
i, j = 3, 5
encrypted = loader[0x60250:]
decoded = bytearray()
for value in encrypted:
    i = (i+2) & 255; j = (j+S[i]+1) & 255
    S[i], S[j] = S[j], S[i]
    decoded.append(value ^ S[(S[i]+S[j]) & 255])
expected = struct.unpack_from('<I', decoded)[0]
stream = zlib.decompressobj()
image = stream.decompress(decoded[4:])
assert stream.eof and len(image) == expected
(OUT / 'jiagu-main-decoded.bin').write_bytes(image)
mask = image[0]; p = 1; parts = []
for _ in range(4):
    size = struct.unpack_from('<I', image, p)[0]; p += 4
    parts.append(bytes(v ^ mask for v in image[p:p+size])); p += size
elf = bytearray(image[p:]); elf[:4] = b'\x7fELF'
assert elf[4:7] == b'\x02\x01\x01'
phoff = struct.unpack_from('<Q', elf, 32)[0]
elf[phoff:phoff+len(parts[0])] = parts[0]
for offset in range(0, len(parts[0]), 56):
    typ, flags, fileoff, vaddr, _, filesz, memsz, align = struct.unpack_from('<IIQQQQQQ', parts[0], offset)
    if typ == 2:
        elf[fileoff:fileoff+filesz] = parts[3] + bytes(filesz-len(parts[3]))
dyn = dict(struct.unpack_from('<QQ', parts[3], p) for p in range(0,len(parts[3]),16))
for address, data in [(dyn[23],parts[1]),(dyn[7],parts[2])]:
    # Both relocation tables are in the first PT_LOAD, where fileoff==vaddr.
    elf[address:address+len(data)] = data
# Section table was removed by the packer; program headers and dynamic table
# suffice for disassembly and analysis. Do not advertise invalid sections.
struct.pack_into('<Q', elf, 40, 0)
struct.pack_into('<HHH', elf, 58, 0, 0, 0)
(OUT / 'jiagu-main-restored.so').write_bytes(elf)
print(json.dumps({'shell_end':shell_end, 'configuration_entries':len(settings),
    'business_blocks':blocks, 'inner_loader_size':len(elf),
    'status':'inner loader recovered; business DEX decryption pending'}, ensure_ascii=False,indent=2))
