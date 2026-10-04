"""Build standalone Go CLI binaries for the three requested operating systems."""
import os, pathlib, subprocess
root=pathlib.Path(__file__).resolve().parents[1]
out=root/'build/bin';out.mkdir(parents=True,exist_ok=True)
for system in ['windows','linux','darwin']:
    for arch in ['amd64','arm64']:
        suffix='.exe' if system=='windows' else ''
        target=out/f'globalspeed-{system}-{arch}{suffix}'
        env={**os.environ,'GOOS':system,'GOARCH':arch,'CGO_ENABLED':'0'}
        subprocess.run(['go','build','-buildvcs=false','-trimpath','-ldflags=-s -w','-o',str(target),'./cmd/globalspeed'],cwd=root,env=env,check=True)
        print(target.name,flush=True)
