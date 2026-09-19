package selftest
import("testing";"unsafe")
type keybdInput struct { Vk,Scan uint16; Flags,Time uint32; ExtraInfo uintptr }
type input struct { Type uint32; _ uint32; Ki keybdInput; _ [8]byte }
func TestWin64InputStructure(t *testing.T){ if unsafe.Sizeof(input{})!=40 {t.Fatalf("INPUT size=%d",unsafe.Sizeof(input{}))}}
func TestSymbolMapping(t *testing.T){
 checks:=[]struct{scan uint32;shift bool;phys,base,want string;ok bool}{
  {0x03,true,"JIS","US","\"",true},{0x07,true,"JIS","US","&",true},
  {0x08,true,"JIS","US","'",true},{0x09,true,"JIS","US","(",true},
  {0x0a,true,"JIS","US",")",true},{0x0b,true,"JIS","US","",true},
  {0x0d,false,"JIS","US","^",true},{0x1a,false,"JIS","US","@",true},
  {0x73,true,"JIS","US","_",true},{0x7d,false,"JIS","US","\\",true},
  {0x03,true,"US","JIS","@",true},{0x07,true,"US","JIS","^",true},
  {0x0c,true,"US","JIS","_",true},{0x29,true,"US","JIS","~",true},
  {0x03,true,"US","US","",false},{0x03,false,"JIS","US","",false},
 }
 for _,c:=range checks { got,ok:=mappedCharacter(c.scan,c.shift,c.phys,c.base);if got!=c.want||ok!=c.ok {t.Errorf("sc=%02X shift=%v %s=>%s got=(%q,%v) expected=(%q,%v)",c.scan,c.shift,c.phys,c.base,got,ok,c.want,c.ok)}}
}
