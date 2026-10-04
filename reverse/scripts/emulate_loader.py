"""Minimal offline AArch64 harness for recovered Jiagu routines.

Not an Android emulator. Stops on unknown imports instead of fabricating results.
"""
import pathlib, sys, struct
ROOT = pathlib.Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'reverse/.tools'))
from unicorn import Uc, UC_ARCH_ARM64, UC_MODE_ARM, UC_HOOK_CODE, UC_HOOK_MEM_INVALID
from unicorn.arm64_const import *

def u32(b,p): return struct.unpack_from('<I',b,p)[0]
def u64(b,p): return struct.unpack_from('<Q',b,p)[0]

class Elf:
    def __init__(self, path, base):
        self.data = path.read_bytes(); self.base = base
        self.ph = [struct.unpack_from('<IIQQQQQQ', self.data, u64(self.data,32)+56*i) for i in range(struct.unpack_from('<H',self.data,56)[0])]
        def offset(va):
            for t,fl,fo,v,p,fs,ms,al in self.ph:
                if t == 1 and v <= va < v+fs: return fo+va-v
            raise ValueError(hex(va))
        self.offset = offset
        dynamic = next(ph for ph in self.ph if ph[0]==2)
        self.dt = {}
        for p in range(dynamic[2], dynamic[2]+dynamic[5],16):
            tag,val = struct.unpack_from('<QQ',self.data,p)
            if tag==0: break
            self.dt[tag]=val
        if 4 in self.dt:
            count = u32(self.data,offset(self.dt[4])+4)
        else:
            shoff=u64(self.data,40)
            sections=[struct.unpack_from('<IIQQQQIIQQ',self.data,shoff+64*i)
                      for i in range(struct.unpack_from('<H',self.data,60)[0])]
            section=next(s for s in sections if s[1]==11)
            count=section[5]//section[9]
        strings = offset(self.dt[5]); syms=offset(self.dt[6])
        self.symbols=[]
        for i in range(count):
            no,info,other,ndx,val,size=struct.unpack_from('<IBBHQQ',self.data,syms+i*24)
            p=strings+no; end=self.data.find(b'\0',p)
            self.symbols.append((self.data[p:end].decode(errors='replace'),val,ndx,size))
        self.reloc=[]
        for tag,size_tag in [(7,8),(23,2)]:
            start=offset(self.dt[tag])
            self.reloc.extend(struct.unpack_from('<QQq',self.data,p) for p in range(start,start+self.dt[size_tag],24))

class Emulator:
    def __init__(self, core=False):
        self.uc=Uc(UC_ARCH_ARM64,UC_MODE_ARM)
        self.images=[Elf(ROOT/'reverse/analysis/jiagu-main-restored.so',0x10000000),Elf(ROOT/'reverse/analysis/libjiagu_a64.so',0x20000000)]
        if core:self.images.append(Elf(ROOT/'reverse/analysis/libGSCore.so',0x30000000))
        self.heap=0x40000000;self.extern=0x70000000;self.functions={};self.exports={}
        self.uc.mem_map(self.heap,64*1024*1024)
        self.uc.mem_map(0x50000000,4*1024*1024)
        self.uc.mem_map(0x51000000,65536)
        self.uc.mem_map(0x60000000,65536)
        self.uc.mem_map(self.extern,1024*1024)
        self.uc.reg_write(UC_ARM64_REG_TPIDR_EL0,0x51000000)
        for img in self.images:
            maxva=max(ph[3]+ph[6] for ph in img.ph if ph[0]==1)
            self.uc.mem_map(img.base,((maxva+4095)//4096)*4096)
            for t,fl,fo,v,p,fs,ms,al in img.ph:
                if t==1:self.uc.mem_write(img.base+v,img.data[fo:fo+fs])
            for name,val,ndx,size in img.symbols:
                if ndx and val: self.exports.setdefault(name,img.base+val)
        for img in self.images:
            for target,info,add in img.reloc:
                typ=info&0xffffffff;si=info>>32
                if typ==1027: value=img.base+add
                elif typ in [1025,1026,257]:
                    name,val,ndx,size=img.symbols[si]
                    value=(img.base+val if ndx else self.resolve(name))+add
                else: raise ValueError('unsupported relocation '+str(typ))
                self.uc.mem_write(img.base+target,struct.pack('<Q',value))
        self.uc.hook_add(UC_HOOK_CODE,self.hook)
        self.last=0
        self.calls=[]
    def resolve(self,name):
        if name in self.exports:return self.exports[name]
        for address,n in self.functions.items():
            if n==name:return address
        address=self.extern;self.extern+=16
        self.functions[address]=name
        return address
    def allocate(self,size):
        size=max(16,size)
        if size>32*1024*1024:raise RuntimeError('allocation too large '+str(size))
        address=self.heap;self.heap+=(size+15)&~15
        if self.heap>=0x44000000:raise RuntimeError('heap exhausted')
        return address
    def string(self,address):
        out=bytearray()
        while address and len(out)<100000:
            v=self.uc.mem_read(address,1)[0]
            if not v:break
            out.append(v);address+=1
        return bytes(out)
    def ret(self,value=0):
        self.uc.reg_write(UC_ARM64_REG_X0,value&0xffffffffffffffff)
        self.uc.reg_write(UC_ARM64_REG_PC,self.uc.reg_read(UC_ARM64_REG_LR))
    def hook(self,uc,address,size,data):
        self.last=address
        name=self.functions.get(address)
        if not name:return
        args=[uc.reg_read(UC_ARM64_REG_X0+i) for i in range(8)]
        a,b,c=args[:3]
        self.calls.append((name,args[:4]))
        if name in ['malloc','_Znwm','_Znam']:
            self.ret(self.allocate(a))
        elif name=='calloc':self.ret(self.allocate(a*b))
        elif name in ['free','_ZdlPv','_ZdaPv','__cxa_atexit','__cxa_finalize','pthread_mutex_lock','pthread_mutex_unlock','pthread_rwlock_rdlock','pthread_rwlock_wrlock','pthread_rwlock_unlock']:
            self.ret()
        elif name in ['memcpy','memmove','__memcpy_chk','__memmove_chk']:
            uc.mem_write(a,bytes(uc.mem_read(b,c)));self.ret(a)
        elif name in ['memset','__memset_chk']:
            uc.mem_write(a,bytes([b&255])*c);self.ret(a)
        elif name=='strlen':self.ret(len(self.string(a)))
        elif name in ['strcmp','strncmp','memcmp','bcmp']:
            x,y=(self.string(a),self.string(b)) if name in ['strcmp','strncmp'] else (bytes(uc.mem_read(a,c)),bytes(uc.mem_read(b,c)))
            if name=='strncmp':x,y=x[:c],y[:c]
            self.ret((x>y)-(x<y))
        elif name in ['strcpy','strncpy']:
            value=self.string(b)+b'\0'
            if name=='strncpy':value=value[:c].ljust(c,b'\0')
            uc.mem_write(a,value);self.ret(a)
        elif name=='strchr':
            v=self.string(a);p=v.find(bytes([b&255]));self.ret(a+p if p>=0 else 0)
        elif name=='memchr':
            p=bytes(uc.mem_read(a,c)).find(bytes([b&255]))
            self.ret(a+p if p>=0 else 0)
        elif name=='__vsprintf_chk' and self.string(args[3])==b'%02x':
            va=args[4]
            stack,grtop,vrtop,groff,vroff=struct.unpack('<QQQii',uc.mem_read(va,32))
            pointer=grtop+groff if groff<0 else stack
            value=struct.unpack('<I',uc.mem_read(pointer,4))[0]
            out=f'{value:02x}'.encode()
            uc.mem_write(a,out+b'\0');self.ret(len(out))
        elif name=='atoi':self.ret(int(self.string(a) or b'0'))
        elif name=='__android_log_print':self.ret()
        elif name=='puts':
            print('Loader log:', self.string(a).decode(errors='replace'))
            self.ret(len(self.string(a))+1)
        else:
            raise RuntimeError(f'Unknown import {name} at {address:#x}, LR={uc.reg_read(UC_ARM64_REG_LR):#x}, args={args[:4]}')
    def call(self,address,args,limit=10_000_000):
        self.uc.reg_write(UC_ARM64_REG_SP,0x503ff000)
        self.uc.reg_write(UC_ARM64_REG_LR,0x60000000)
        for i,a in enumerate(args):self.uc.reg_write(UC_ARM64_REG_X0+i,a)
        self.uc.emu_start(address,0x60000000,count=limit)
        if self.uc.reg_read(UC_ARM64_REG_PC)!=0x60000000:raise RuntimeError(f'Instruction limit; PC={self.last:#x}')
        return self.uc.reg_read(UC_ARM64_REG_X0)

if __name__=='__main__':
    emu=Emulator()
    # Reconstruct the cross-module bridges. The makekey ABI matches the
    # outer exported __arm_a_2(char*, size_t, char*, int&, int).
    # These are hypotheses pending validation against a recovered DEX.
    for target, source in [(0x1917b0,0x10198),(0x19a008,0x471d0),
                           (0x19a010,0x472ac),(0x19a018,0x47348)]:
        emu.uc.mem_write(0x10000000+target,struct.pack('<Q',0x20000000+source))
    obj=emu.allocate(0x98)
    emu.call(0x1005f4b4,[obj])
    import zipfile
    z=zipfile.ZipFile(ROOT/'reverse/samples/globalspeed_4.4.8_safe.apk')
    blob=z.read('classes.dex')[15948+12:15948+1288]
    data=emu.allocate(len(blob));emu.uc.mem_write(data,blob)
    try:
        result=emu.call(0x1005fac8,[obj,data,len(blob)])
        key=bytes(emu.uc.mem_read(obj+10,16))
        print('RESULT',result,'KEY',key.hex())
        (ROOT/'reverse/analysis/business-key.bin').write_bytes(key)
    except Exception:
        print('LAST PC',hex(emu.last),'last imports',emu.calls[-12:])
        raise
