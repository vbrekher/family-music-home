#!/usr/bin/env python3
"""Rasterize the project's original simple vector mark with Pillow (GPLv3 source).

Geometry and palette match ui/public/family-music-home.svg. No downloaded artwork
or image metadata is used. Run locally when intentionally refreshing brand icons.
"""
from pathlib import Path
from PIL import Image, ImageDraw

root = Path(__file__).resolve().parents[1]
scale = 8
image = Image.new('RGB', (128*scale, 128*scale), '#203f3b')
draw = ImageDraw.Draw(image)
def points(items):
    return [(round(x*scale), round(y*scale)) for x,y in items]
def circle(x,y,r,fill,outline=None,width=1):
    draw.ellipse(((x-r)*scale,(y-r)*scale,(x+r)*scale,(y+r)*scale),fill=fill,outline=outline,width=round(width*scale))
draw.polygon(points([(20,61),(64,25),(108,61),(108,110),(20,110)]),fill='#e9cda4')
draw.line(points([(12,61),(64,18),(116,61)]),fill='#ead8bd',width=9*scale,joint='curve')
circle(12,61,4.5,'#ead8bd');circle(116,61,4.5,'#ead8bd')
circle(64,77,26,'#203f3b');circle(64,77,18,None,'#8ea7a0',1.5);circle(64,77,10,'#d89871');circle(64,77,3,'#203f3b')
for p in (root/'ui/public').glob('*.png'):
    with Image.open(p) as old:size=old.size
    canvas=Image.new('RGB',size,'#203f3b');mark=image.copy();mark.thumbnail(size,Image.Resampling.LANCZOS);canvas.paste(mark,((size[0]-mark.width)//2,(size[1]-mark.height)//2));canvas.save(p)
image.resize((192,192),Image.Resampling.LANCZOS).save(root/'resources/logo-192x192.png')
if (root/'ui/public/favicon.ico').exists():
    image.resize((256,256),Image.Resampling.LANCZOS).save(root/'ui/public/favicon.ico',sizes=[(16,16),(32,32),(48,48),(64,64)])
print('Regenerated original project PNG/ICO brand icons without external artwork or metadata.')
