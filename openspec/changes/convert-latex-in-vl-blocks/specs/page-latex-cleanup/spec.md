## Purpose

Reads a text that a visual OCR engine wrote with LaTeX fragments in it, and
answers the plain text of it. It converts the two markup shapes that engine
produces on a printed book, and refuses everything it has no evidence for.

## ADDED Requirements

### Requirement: The cleanup converts a footnote marker

The cleanup SHALL replace `$ ^{33} $` with `³³`, and `$ _{2} $` with `₂`, in
every text it reads. It SHALL accept the shape with the spaces and the shape
without them.

The body of the marker SHALL be digits alone. A body that holds anything else
has no Unicode form for every one of its characters, and the cleanup SHALL leave
such a marker as it stands rather than convert half of it.

The cleanup SHALL join the marker to the word in front of it, as the printed
page does, and SHALL leave the space behind the marker alone.

This conversion SHALL NOT pair the dollar signs of the text. It SHALL match the
whole shape at once, so an unpaired dollar sign elsewhere in the text cannot
reach it, and a price cannot be read as the start of a marker.

#### Scenario: A marker inside a sentence

- **WHEN** the cleanup reads `Con la palabra misma $ ^{1} $ está vinculada`
- **THEN** it answers `Con la palabra misma¹ está vinculada`

#### Scenario: A marker in front of a full stop

- **WHEN** the cleanup reads `que en su lugar se describen $ ^{50} $. Dentro de`
- **THEN** it answers `que en su lugar se describen⁵⁰. Dentro de`

#### Scenario: A marker written without spaces

- **WHEN** the cleanup reads `se ofrece$^{51}$ del rasgo`
- **THEN** it answers `se ofrece⁵¹ del rasgo`

#### Scenario: A subscript marker

- **WHEN** the cleanup reads `el agua $ _{2} $ hierve`
- **THEN** it answers `el agua₂ hierve`

#### Scenario: A marker the cleanup cannot spell

- **WHEN** the cleanup reads `la nota $ ^{a} $ del pie`
- **THEN** it answers `la nota $ ^{a} $ del pie`

#### Scenario: A price is not a marker

- **WHEN** the cleanup reads `it costs $5 and I paid $12`
- **THEN** it answers `it costs $5 and I paid $12`

### Requirement: The cleanup rescues a Greek word only on evidence

The engine reads a Greek word as a mixture of Greek runes and LaTeX Greek
commands, and wraps the pieces in dollar signs. The dollar signs are not the
delimiters of a formula. The engine wraps single words in them, including words
of the surrounding language, and it writes an odd number of them on a text that
holds several.

The cleanup SHALL therefore take the whole text as its unit, and SHALL rescue it
only on evidence. The evidence SHALL be one LaTeX command or more that names a
Greek letter, and nothing else. A printed book holds no LaTeX command, so the
engine wrote every one the cleanup finds.

The runes beside a command SHALL be no part of the evidence. The engine writes a
Greek word with Latin lookalikes when it does not write it as a command: it
answers `éóv`, a Latin e, a Latin o and a Latin v, for a word the book prints
`ἐόν`. A text that reads as Greek to the eye can therefore hold no Greek rune at
all, and a rule that asks for one would leave such a text as it stands.

On a text that holds the evidence, the cleanup SHALL:

- replace every LaTeX Greek command with its Unicode letter,
- answer the letter alone for a command that puts an accent on one, because the
  accent is the guess of an engine that could not place the letter,
- answer the Greek letter for a command the engine writes for a letter it did not
  read as one, and
- remove every dollar sign of that text.

A command of the third kind SHALL NOT be evidence. `\circ` is the ring operator
of real mathematics, so a text that names it and no Greek letter SHALL keep every
dollar sign it holds.

The cleanup SHALL NOT leave the spacing the dollar signs occupied. It SHALL join
the words the removal separated, and SHALL NOT leave a space in front of the
punctuation that follows a rescued word.

On a text that holds no Greek command, the cleanup SHALL leave every dollar sign
of it as it stands. A dollar sign is not evidence of LaTeX, and a Greek rune is
not either.

The cleanup SHALL know the letters of the Greek alphabet, their common LaTeX
variants, the accents above, and no other command. It SHALL leave a command it
does not know as it stands.

#### Scenario: A text of Greek runes and Greek commands

- **WHEN** the cleanup reads
  `es $ ᄋλὴ $ \eta\theta\epsilon ῥ\eta $ o $ τὸ $ $ ἐóν $, pero $ ἱ $ $ cómo $ se llama`
- **THEN** it answers `es ᄋλὴ ηθε ῥη o τὸ ἐóν, pero ἱ cómo se llama`
- **AND** the answer holds no dollar sign
- **AND** the answer holds no backslash

#### Scenario: A text of commands with no Greek rune in it

- **WHEN** the cleanup reads `contraposición de $ \lambda\eta\theta\epsilon\iota\eta $ a
  $ \delta\delta\xi\alpha $ o de $ \tau\ddot{\alpha} $ éóv a
  $ \tau\ddot{\alpha}\delta\kappa\circ\bar{\upsilon}\nu\tau\alpha $.`
- **THEN** it answers `contraposición de ληθειη a δδξα o de τα éóv a ταδκουντα.`
- **AND** it answers so although the text it read holds no Greek rune

#### Scenario: A text whose dollar signs do not pair

- **WHEN** the cleanup reads a paragraph of 27 dollar signs and 12 Greek
  commands, which no pairing can read as delimited spans
- **THEN** the answer holds no dollar sign and no backslash
- **AND** the answer holds the Greek word `μὴ εἴναι` with one space between the
  two words

#### Scenario: Greek runes without a command

- **WHEN** the cleanup reads `un texto con τὸ ἐόν pero $ sin $ comandos`
- **THEN** it answers `un texto con τὸ ἐόν pero $ sin $ comandos`

#### Scenario: One command is evidence enough

- **WHEN** the cleanup reads `la relectura $ \alpha $ del rasgo, sin griego`
- **THEN** it answers `la relectura α del rasgo, sin griego`

#### Scenario: A text of mathematics keeps its dollar signs

- **WHEN** the cleanup reads `the map $ f \circ g $ and the angle $ 30 \circ $`
- **THEN** it answers `the map $ f \circ g $ and the angle $ 30 \circ $`

#### Scenario: A rescued text also gains its markers

- **WHEN** the cleanup reads a text that holds a Greek command and the marker
  `$ ^{12} $`
- **THEN** the answer holds `¹²` joined to the word in front of it
- **AND** the answer holds no dollar sign

#### Scenario: An unknown command survives the rescue

- **WHEN** the cleanup reads `el término $ τὸ $ \eta y \foobar aparte`
- **THEN** it answers `el término τὸ η y \foobar aparte`

### Requirement: The cleanup changes nothing else

The cleanup SHALL answer a text it has no rule for exactly as it read it, rune
for rune. It SHALL NOT convert a fraction, a text-mode wrapper, a display
formula, a bracket delimiter, or any construct beyond the two requirements above.

A text that names no Greek command SHALL come back as it stands, whatever else it
holds. The rescue is the only rule that reads a dollar sign of such a text, and
it never runs on one.

The cleanup SHALL be repeatable. The text it answers SHALL answer itself when
the cleanup reads it again.

The cleanup SHALL read one text and answer one text. It SHALL NOT read a box, a
line, a page or an engine name, and its answer SHALL NOT depend on any of them.

#### Scenario: Ordinary prose

- **WHEN** the cleanup reads `Pues bien, también en la otra contraposición`
- **THEN** it answers `Pues bien, también en la otra contraposición`

#### Scenario: A Windows path

- **WHEN** the cleanup reads `guardado en C:\temp\newfile.txt`
- **THEN** it answers `guardado en C:\temp\newfile.txt`

#### Scenario: A construct the cleanup has no rule for

- **WHEN** the cleanup reads `la razón $\frac{a+b}{2}$ del texto`
- **THEN** it answers `la razón $\frac{a+b}{2}$ del texto`

#### Scenario: A second reading changes nothing

- **WHEN** the cleanup reads its own answer for any text
- **THEN** it answers that text unchanged

#### Scenario: An empty text

- **WHEN** the cleanup reads an empty text
- **THEN** it answers an empty text
