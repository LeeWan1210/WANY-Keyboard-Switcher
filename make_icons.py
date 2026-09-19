"""Generate original tray and Windows executable icon assets, no external images."""
from PIL import Image, ImageDraw, ImageFont
from pathlib import Path
P=Path(__file__).parent
FONT='/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf'

def build(name, bg, accent):
    s=256
    im=Image.new('RGBA',(s,s),(0,0,0,0));d=ImageDraw.Draw(im)
    d.rounded_rectangle((10,10,246,246),radius=52,fill=bg)
    # Stylized bright keyboard frame (keyboard clearly visible at 32px tray size)
    d.rounded_rectangle((33,57,223,178),radius=15,fill='#17263b',outline='#ffffff',width=5)
    for y,xs in [(75,[47,78,109,140,171]),(103,[47,78,109,140,171])]:
        for x in xs:
            d.rounded_rectangle((x,y,x+23,y+17),radius=4,fill='#ebf5ff')
    for x,w in [(47,31),(85,91),(183,26)]:
        d.rounded_rectangle((x,133,x+w,155),radius=5,fill=accent)
    # Small contrasting badge with physical profile
    d.rounded_rectangle((74,163,182,226),radius=19,fill=accent,outline='#ffffff',width=3)
    txt='US' if name=='us' else 'JIS'
    f=ImageFont.truetype(FONT,37 if name=='jis' else 44)
    bbox=d.textbbox((0,0),txt,font=f)
    d.text(((256-(bbox[2]-bbox[0]))/2,191-(bbox[3]-bbox[1])/2-bbox[1]),txt,font=f,fill='#ffffff')
    im.save(P/f'icon_{name}.png')
    im.save(P/f'icon_{name}.ico',format='ICO',sizes=[(16,16),(24,24),(32,32),(48,48),(64,64),(128,128),(256,256)])
    return im
build('us','#1765b0','#11528f')
build('jis','#a64c16','#a44b15')
# The Explorer icon is intentionally neutral: keyboard pictogram without JIS/US mode badge.
im=Image.new('RGBA',(256,256),(0,0,0,0));d=ImageDraw.Draw(im)
d.rounded_rectangle((10,10,246,246),radius=52,fill='#146c8d')
d.rounded_rectangle((30,65,226,190),radius=17,fill='#17314e',outline='#ffffff',width=6)
for y in (82,111):
 for x in (46,77,108,139,170,201):
  if x+20<219:d.rounded_rectangle((x,y,x+20,y+19),radius=4,fill='#eef8ff')
d.rounded_rectangle((50,147,205,171),radius=5,fill='#eff7ff')
im.save(P/'icon_app.png')
im.save(P/'icon_app.ico',format='ICO',sizes=[(16,16),(24,24),(32,32),(48,48),(64,64),(128,128),(256,256)])
print('icons created')
