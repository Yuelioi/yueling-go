# Word-cloud test font

`ChatCloudTest.ttf` is a renamed subset of Noto Sans SC Regular by Adobe, licensed under [SIL OFL 1.1](OFL.txt). It retains ASCII and the CJK characters referenced by the word-cloud renderer and its tests. The fixture lets clean checkouts test Chinese glyph bounds without local data files or system font discovery.

- `ChatCloudTest.ttf` — Chinese glyph and layout fixture, used only by tests.
- `OFL.txt` — Upstream copyright notice and font redistribution license, from [Google Fonts](https://github.com/google/fonts/blob/main/ofl/notosanssc/OFL.txt).
- `generate.py` — Regenerates the subset with Python fonttools. Run `python generate.py /path/to/NotoSansSC-Regular.ttf` after adding test characters. The original font is [Noto Sans SC](https://fonts.google.com/noto/specimen/Noto+Sans+SC); do not use the test subset as a production font.
