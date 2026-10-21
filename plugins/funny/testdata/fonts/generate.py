"""Create the OFL test font from NotoSansSC-Regular.ttf with fonttools.

Run from this directory: python generate.py /path/to/NotoSansSC-Regular.ttf
The source is Noto Sans SC (Adobe), https://fonts.google.com/noto/specimen/Noto+Sans+SC.
Only ASCII and characters used by the word-cloud regression tests are retained.
"""
from pathlib import Path
import sys
from fontTools import subset
from fontTools.ttLib import TTFont

root = Path(__file__).resolve().parents[2]
text = ''.join((root / name).read_text() for name in [
    'chatstats_test.go', 'chatstats_image_test.go', 'chatstats_image.go',
])
characters = {c for c in text if '\u4e00' <= c <= '\u9fff'}
characters.update(chr(i) for i in range(32, 127))
font = TTFont(sys.argv[1])
options = subset.Options()
options.hinting = True
options.name_IDs = ['*']
subsetter = subset.Subsetter(options=options)
subsetter.populate(text=''.join(sorted(characters)))
subsetter.subset(font)
for record in font['name'].names:
    names = {1: 'Chat Cloud Test', 2: 'Regular', 3: 'ChatCloudTest-Regular',
             4: 'Chat Cloud Test Regular', 6: 'ChatCloudTest-Regular',
             16: 'Chat Cloud Test', 17: 'Regular'}
    if record.nameID in names:
        record.string = names[record.nameID].encode(record.getEncoding())
font.save(Path(__file__).with_name('ChatCloudTest.ttf'))
