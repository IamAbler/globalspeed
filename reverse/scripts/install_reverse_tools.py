"""Install local analysis wheels in /tmp without modifying system Python."""
import json, pathlib, urllib.request, zipfile, io

dest = pathlib.Path(__file__).resolve().parents[2] / 'reverse/.tools'
dest.mkdir(exist_ok=True)
for package in ['capstone', 'unicorn']:
    info = json.load(urllib.request.urlopen('https://pypi.org/pypi/' + package + '/json'))
    candidates = [f for f in info['urls'] if f['filename'].endswith('.whl')
        and ('manylinux' in f['filename']) and 'x86_64' in f['filename']
        and ('py3-none' in f['filename'] or 'py2.py3-none' in f['filename'] or 'abi3' in f['filename'])]
    if not candidates: raise RuntimeError('No compatible wheel for ' + package)
    wheel = candidates[0]
    blob = urllib.request.urlopen(wheel['url']).read()
    import hashlib
    assert hashlib.sha256(blob).hexdigest() == wheel['digests']['sha256']
    with zipfile.ZipFile(io.BytesIO(blob)) as archive: archive.extractall(dest)
    print(package, info['info']['version'], wheel['filename'])
