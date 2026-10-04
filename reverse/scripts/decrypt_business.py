"""Recover this APK's three business DEX images from the offline loader key.

Run unpack_layers.py and emulate_loader.py first. Never executes Android code.
"""
import hashlib, json, lzma, pathlib, struct, zlib

ROOT = pathlib.Path(__file__).resolve().parents[2]
OUT = ROOT / 'reverse/analysis'
key = (OUT / 'business-key.bin').read_bytes()
assert len(key) == 16
results = []
for number in range(1, 4):
    block = (OUT / f'business-block-{number}.bin').read_bytes()
    size = struct.unpack_from('<I', block)[0]
    state = list(range(256)); j = 0
    for i in range(256):
        j = (j + state[i] + key[i % 16]) & 255
        state[i], state[j] = state[j], state[i]
    i, j = 3, 5
    plain = bytearray()
    for value in block[4:4+size]:
        i = (i + 2) & 255; j = (j + state[i] + 1) & 255
        state[i], state[j] = state[j], state[i]
        plain.append(value ^ state[(state[i] + state[j]) & 255])
    prop = plain[0]
    dictionary, expected, compressed = struct.unpack_from('<III', plain, 1)
    assert compressed == size - 13
    decoder = lzma.LZMADecompressor(format=lzma.FORMAT_RAW, filters=[{
        'id': lzma.FILTER_LZMA1, 'dict_size': dictionary,
        'lc': prop % 9, 'lp': (prop // 9) % 5, 'pb': prop // 45}])
    prefix = decoder.decompress(plain[13:])
    assert len(prefix) == expected
    image = prefix + block[4+size:]
    # Only the 112-byte DEX header carries this additional XOR layer.
    dex = bytes(value ^ 0x17 for value in image[:112]) + image[112:]
    assert dex[:8] == b'dex\n037\0'
    assert struct.unpack_from('<I', dex, 32)[0] == len(dex)
    assert struct.unpack_from('<I', dex, 8)[0] == zlib.adler32(dex[12:]) & 0xffffffff
    path = OUT / f'classes{number}-recovered.dex'
    path.write_bytes(dex)
    count, offset = struct.unpack_from('<II', dex, 56)
    strings = []
    for index in range(count):
        p = struct.unpack_from('<I', dex, offset + 4 * index)[0]
        while dex[p] & 128: p += 1
        p += 1
        strings.append(dex[p:dex.index(0, p)].decode(errors='replace'))
    (OUT / f'classes{number}-strings.txt').write_text('\n'.join(strings))
    results.append({'file': path.name, 'size': len(dex), 'string_count': count,
                    'adler32_valid': True,
                    'header_sha1_valid': hashlib.sha1(dex[32:]).digest() == dex[12:32],
                    'sha256': hashlib.sha256(dex).hexdigest()})
(OUT / 'recovered-dex.json').write_text(json.dumps(results, indent=2))
print(json.dumps(results, indent=2))
