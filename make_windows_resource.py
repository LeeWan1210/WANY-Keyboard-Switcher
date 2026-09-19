"""Build a PE/COFF icon resource for Go windows/amd64 using only stdlib."""
import pathlib, struct
P=pathlib.Path(__file__).parent
ico=(P/'icon_app.ico').read_bytes()
reserved,kind,count=struct.unpack_from('<HHH',ico)
assert reserved==0 and kind==1
images=[];entries=[]
for i in range(count):
  w,h,nc,res,planes,bits,size,offset=struct.unpack_from('<BBBBHHII',ico,6+16*i)
  img=ico[offset:offset+size]
  assert len(img)==size
  images.append(img)
  entries.append((w,h,nc,res,planes,bits,size,i+1))
group=struct.pack('<HHH',0,1,count)+b''.join(struct.pack('<BBBBHHIH',*e) for e in entries)
resources=[(3,i+1,img) for i,img in enumerate(images)]+[(14,1,group)]
# root -> type -> numeric resource ID -> language -> IMAGE_RESOURCE_DATA_ENTRY
buf=bytearray(); reloc=[]
def alloc(n,alignment=4):
 while len(buf)%alignment:buf.append(0)
 off=len(buf);buf.extend(b'\0'*n);return off
def directory(items):
 off=alloc(16+8*len(items))
 struct.pack_into('<IIHHHH',buf,off,0,0,0,0,0,len(items))
 for i,key in enumerate(items):struct.pack_into('<I',buf,off+16+8*i,int(key))
 return off
def point_dir(parent,slot,child):struct.pack_into('<I',buf,parent+16+8*slot+4,0x80000000|child)
def point_data(parent,slot,child):struct.pack_into('<I',buf,parent+16+8*slot+4,child)
root=directory([3,14]);rtype_dirs={}
for slot,typ in enumerate((3,14)):
 ids=[rid for t,rid,_ in resources if t==typ]; tdir=directory(ids);rtype_dirs[typ]=tdir;point_dir(root,slot,tdir)
 for j,rid in enumerate(ids):
  langdir=directory([0x0409]);point_dir(tdir,j,langdir)
  dataentry=alloc(16);point_data(langdir,0,dataentry)
  payload=next(b for t,r,b in resources if t==typ and r==rid)
  poff=alloc(len(payload));buf[poff:poff+len(payload)]=payload
  struct.pack_into('<IIII',buf,dataentry,poff,len(payload),0,0)
  reloc.append(dataentry)
# COFF header + section header + raw data + relocations + section symbol
raw=bytes(buf);raw_offset=20+40;reloc_offset=raw_offset+len(raw);sym_offset=reloc_offset+10*len(reloc)
coff=bytearray()
coff+=struct.pack('<HHIIIHH',0x8664,1,0,sym_offset,1,0,0)
coff+=struct.pack('<8sIIIIIIHHI',b'.rsrc\0\0\0',0,0,len(raw),raw_offset,reloc_offset,0,len(reloc),0,0x40000040)
coff+=raw
for off in reloc:coff+=struct.pack('<IIH',off,0,3) # IMAGE_REL_AMD64_ADDR32NB -> .rsrc symbol
coff+=struct.pack('<8sIhHBB',b'.rsrc\0\0\0',0,1,0,3,0)
coff+=struct.pack('<I',4) # empty string table
(P/'rsrc_windows_amd64.syso').write_bytes(coff)
print('resource:',len(coff),'bytes, icon images:',count,'relocations:',len(reloc))
