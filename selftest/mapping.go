package selftest
func mappedCharacter(sc uint32,shift bool,physical,baseline string)(string,bool){
 if physical==baseline{return "",false}
 if physical=="JIS" && baseline=="US" {
  switch sc {
  case 0x03:if shift{return "\"",true}
  case 0x07:if shift{return "&",true}
  case 0x08:if shift{return "'",true}
  case 0x09:if shift{return "(",true}
  case 0x0A:if shift{return ")",true}
  case 0x0B:if shift{return "",true} // JIS Shift+0 prints no character.
  case 0x0C:if shift{return "=",true}
  case 0x0D:if shift{return "~",true};return "^",true
  case 0x1A:if shift{return "`",true};return "@",true
  case 0x1B:if shift{return "{",true};return "[",true
  case 0x27:if shift{return "+",true}
  case 0x28:if shift{return "*",true};return ":",true
  case 0x2B:if shift{return "}",true};return "]",true
  case 0x73:if shift{return "_",true};return "\\",true // JIS Ro key, if firmware supplies SC073
  case 0x7D:if shift{return "|",true};return "\\",true // JIS Yen key; often backslash in code
  }
 }
 if physical=="US" && baseline=="JIS" {
  switch sc {
  case 0x03:if shift{return "@",true}
  case 0x07:if shift{return "^",true}
  case 0x08:if shift{return "&",true}
  case 0x09:if shift{return "*",true}
  case 0x0A:if shift{return "(",true}
  case 0x0B:if shift{return ")",true}
  case 0x0C:if shift{return "_",true}
  case 0x0D:if shift{return "+",true};return "=",true
  case 0x1A:if shift{return "{",true};return "[",true
  case 0x1B:if shift{return "}",true};return "]",true
  case 0x27:if shift{return ":",true}
  case 0x28:if shift{return "\"",true};return "'",true
  case 0x2B:if shift{return "|",true};return "\\",true
  case 0x29:if shift{return "~",true};return "`",true // US grave key vs JP 半角/全角
  }
 }
 return "",false
}
