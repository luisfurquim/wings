# NOTICE — LGPLLR Linguistic Dictionaries

The dictionaries listed here are downloaded on demand by `cmd/dictbuild`
from the [UnitexGramLab/unitex-lingua](https://github.com/UnitexGramLab/unitex-lingua)
repository and are **not stored in this repository**. They are cached
locally under `$XDG_CACHE_HOME/wprana/dictbuild/` (or the equivalent
platform path) and are never bundled into WPrana's WASM binary.

All dictionaries are distributed under the
**Lesser General Public License for Linguistic Resources (LGPLLR)**.
The full license text is at [`LICENSES/LGPLLR.txt`](../../LICENSES/LGPLLR.txt)
from the repository root.

Copyright for all entries: © The Unitex/GramLab authors and contributors
(<https://unitexgramlab.org>).

---

## Modification notice (LGPLLR Section 2b)

`cmd/dictbuild` reconstitutes each compiled `.bin`+`.inf` pair into a
UTF-16 text DELAF using `UnitexToolLogger Uncompress`, then parses it
into a compact two-layer lookup structure (Lemmas + FormIndex, encoded
as a Go gob file `<lang>.db`). This transformation constitutes a
modification of the original Linguistic Resource.

**Changed by:** WPrana `cmd/dictbuild`  
**Date:** 2026-04-27  
**Nature of change:** decompression from binary DAWG to text DELAF,
followed by parsing into a language-neutral `Dict{Lemmas, FormIndex}`
structure; proper names (+Pr), enclitic pronouns (+PRO), imperative
forms, and 1st/2nd person finite verbal forms are filtered out.

---

## Per-language entries

### de — German

- **File:** `de/Dela/dela.bin` + `dela.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** general DELAF

### el — Greek (partial)

- **File:** `el/Dela/dela-30percent.bin` + `dela-30percent.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** 30% sample dictionary; coverage is partial

### en — English

- **File:** `en/Dela/dela-en-public.bin` + `dela-en-public.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** public DELAF

### es — Spanish

- **File:** `es/Dela/delaf.bin` + `delaf.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** general DELAF

### fi — Finnish

- **File:** `fi/Dela/pien_DELAF_sanasto.bin` + `pien_DELAF_sanasto.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** general DELAF

### fr — French

- **File:** `fr/Dela/Dela_fr.bin` + `Dela_fr.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** general DELAF

### grc — Ancient Greek

- **File:** `grc/Dela/AG_demo_dico.bin` + `AG_demo_dico.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** demo dictionary; coverage is partial

### it — Italian

- **File:** `it/Dela/mini-delaf.bin` + `mini-delaf.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** mini DELAF; coverage is partial

### la — Latin

- **File:** `la/Dela/perseus-lewis-short.bin` + `perseus-lewis-short.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** based on Perseus Lewis & Short

### mg — Malagasy

- **File:** `mg/Dela/free-DEMA-VSflx.bin` + `free-DEMA-VSflx.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** verb-stem flexion dictionary

### nn — Norwegian Nynorsk (sample)

- **File:** `nn/Dela/Dela-sample.bin` + `Dela-sample.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** sample dictionary; coverage is partial

### no — Norwegian Bokmål

- **File:** `no/Dela/2011+proprium.bin` + `2011+proprium.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** 2011 edition including proper nouns

### oge — Old Georgian

- **File:** `oge/Dela/Georgian (Ancient)_may2009.bin` + `.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** May 2009 edition

### pl — Polish

- **File:** `pl/Dela/dela_pl-.bin` + `dela_pl-.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** general DELAF

### pt-BR — Brazilian Portuguese

- **File:** `pt-BR/Dela/DELAF_PB_2018.bin` + `DELAF_PB_2018.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** 2018 edition; ~589,625 entries (after dictbuild filters)

### pt-PT — European Portuguese

- **File:** `pt-PT/Dela/Delaf_V2.bin` + `Delaf_V2.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** version 2

### ru — Russian

- **File:** `ru/Dela/CISLEXru_igrok.bin` + `CISLEXru_igrok.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** CISLEX Russian dictionary

### sr-Cyrl — Serbian (Cyrillic)

- **File:** `sr-Cyrl/Dela/cirdelaf-SrpskiU.bin` + `cirdelaf-SrpskiU.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** Cyrillic script DELAF

### sr-Latn — Serbian (Latin)

- **File:** `sr-Latn/Dela/latdelaf-SrpskiU.bin` + `latdelaf-SrpskiU.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** Latin script DELAF

### th — Thai

- **File:** `th/Dela/dela.bin` + `dela.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** general DELAF

### zh — Chinese

- **File:** `zh/Dela/segdic_unitex_pinyin_2017.bin` + `segdic_unitex_pinyin_2017.inf`
- **Copyright:** © The Unitex/GramLab authors and contributors
- **Note:** Pinyin segmentation dictionary, 2017 edition
