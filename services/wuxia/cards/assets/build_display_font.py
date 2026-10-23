import argparse
from pathlib import Path

from fontTools import subset
from fontTools.ttLib import TTFont

parser = argparse.ArgumentParser(description="Build the game's embedded display font from official Noto Serif sources")
parser.add_argument("source_font", type=Path)
parser.add_argument("source_license", type=Path)
args = parser.parse_args()
assets = Path(__file__).resolve().parent
font = TTFont(args.source_font, recalcTimestamp=False)
notice = "\n".join(sorted({record.toUnicode() for record in font["name"].names if record.nameID == 0}))
options = subset.Options()
options.name_IDs = ["*"]
options.name_legacy = True
options.name_languages = ["*"]
options.notdef_glyph = True
builder = subset.Subsetter(options=options)
builder.populate(text=(assets / "display-glyphs.txt").read_text())
builder.subset(font)
names = {1: "Yueling UI Display", 4: "Yueling UI Display SemiBold", 6: "YuelingUIDisplay-SemiBold", 16: "Yueling UI Display", 17: "SemiBold"}
for record in font["name"].names:
    if record.nameID in names:
        record.string = names[record.nameID].encode(record.getEncoding())
if "CFF " in font:
    cff = font["CFF "].cff
    cff.fontNames = ["YuelingUIDisplay-SemiBold"]
    cff.topDictIndex[0].FamilyName = "Yueling UI Display"
    cff.topDictIndex[0].FullName = "Yueling UI Display SemiBold"
font.save(assets / "ui-display.otf")
(assets / "OFL.txt").write_text(notice + "\n\n" + args.source_license.read_text())
