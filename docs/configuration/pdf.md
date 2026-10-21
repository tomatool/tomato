# PDF configuration

The `pdf` resource asserts on what is **inside** the PDF documents your
application generates: the page count, the text (on the document, on one
page, and the order of two pieces of it), the metadata, the outline, and
whether the document is accessible: tagged, with a language and a title,
and conforming to PDF/UA or PDF/A. It reads a document from a file, from an
object of an `s3` resource, or from the last response of an `http` resource.

`s3` on its own can say an object exists, its content type and its size.
The text of a PDF sits in compressed content streams, so `content should
contain` never matches it; this resource reads the document instead.

```yaml
version: 2

containers:
  minio:
    image: cgr.dev/chainguard/minio:latest
    command: ["server", "/data"]
    ports:
      - "9000/tcp"
  verapdf:
    preset: verapdf        # only needed for the conformance step

app:
  command: go run ./cmd/server
  port: 8080

resources:
  agreements-storage:
    type: s3
    container: minio
    options:
      buckets:
        - cas
  app-client:
    type: http-client
    base_url: http://localhost:8080
  agreement-pdf:
    type: pdf
    options:
      validator: verapdf   # optional: the container that checks PDF/UA and PDF/A
```

```gherkin
Scenario: A deselected clause keeps its number in the country PDF
  When "app-client" sends "POST" to "/versions/AT/pdf"
  Then "agreements-storage" object "cas/CA_Version_No._07.07_AT-de.pdf" should exist within "30s"
  When "agreement-pdf" reads "agreements-storage" object "cas/CA_Version_No._07.07_AT-de.pdf"
  Then "agreement-pdf" text contains "9 Kryptowerte"
  And "agreement-pdf" text contains "Absichtlich frei gelassen"
  And "agreement-pdf" text contains "Version 07.07"
  And "agreement-pdf" text does not contain "{{"

Scenario: A bilingual version is sequential, not columns
  When "agreement-pdf" reads "agreements-storage" object "cas/CA_Version_No._07.07_LU-fr-en.pdf"
  Then "agreement-pdf" text "Conditions générales" appears before "General Terms and Conditions"

Scenario: The generated PDF is accessible
  When "app-client" sends "GET" to "/versions/AT/pdf"
  And "agreement-pdf" reads "app-client" response body
  Then "agreement-pdf" is tagged
  And "agreement-pdf" language is "de-AT"
  And "agreement-pdf" metadata "Title" is "Kundenvereinbarung"
  And "agreement-pdf" conforms to "PDF/UA-1"
```

## Options

| Option | Default | Meaning |
|--------|---------|---------|
| `validator` | none | The name of a container running veraPDF, normally one with `preset: verapdf`. Only `conforms to` needs it; without it that step fails and says so, and every other step works. |
| `timeout` | `2m` | How long one conformance check may take. veraPDF needs a few seconds for a long document, more under emulation. |

## Reading a document

Three steps load a document; every other step asserts on the last one
loaded.

| Step | Reads |
|------|-------|
| `"agreement-pdf" reads file "out/ca.pdf"` | A file, relative to the directory tomato runs in, as `shell` and the `s3` upload steps resolve paths. |
| `"agreement-pdf" reads "agreements-storage" object "cas/x.pdf"` | An object of the `s3` resource named `agreements-storage`, as `bucket/key`, the same form that resource's own steps take. |
| `"agreement-pdf" reads "app-client" response body` | The body of the last response the `http` resource named `app-client` received. Send the request first. |

The resource named in a step has to be of the right type: naming a `redis`
resource in `reads "app-cache" object` fails with what it is. A document that
does not parse fails the reading step with pdfcpu's reason, and the previous
document stays loaded.

Before every scenario, reset drops the loaded document. Nothing else is
kept or cleared; the validator container stays up.

## Text

tomato reads the text out of the content streams itself. It follows the
document the way a viewer draws it, font by font, and rebuilds words and
lines from where the glyphs land rather than from how the generator split
the runs: Word writes a line as one run per kerning pair, fpdf and TCPDF
one run per cell, and both read back as lines of words. The text of each
page is lines separated by newlines; the document's text is its pages in
order.

Matching is exact and case-sensitive, but whitespace is not significant: a
phrase that breaks across a line, or that the generator spaced differently,
matches what a person reads. `text contains "9 Kryptowerte"` finds it on
any page; `page "3" contains` looks at one page, counted from 1; `text
"Conditions générales" appears before "General Terms and Conditions"` fails
when either is missing, and names the page the second one is on when the
order is wrong. A failed assertion shows how many pages and characters were
read and the start of the text.

What it does not do:

- **Reading order across columns.** Text comes in the order the document
  draws it, which is reading order for generated documents and may
  interleave columns in a magazine layout.
- **Hyphenation.** A word broken at a line end stays broken: `cor-` and
  `ners` on two lines.
- **Ligatures.** `ﬁ` reads as `ﬁ` or as `fi`, whichever the font's ToUnicode
  map says. Assert on text around them when a document's font keeps them.
- **Glyphs with no text behind them.** A font without a ToUnicode map (icon
  fonts, some subsetted CID fonts) reads as `�` per glyph.
- **Annotations, form fields and images.** Text in a form field's
  appearance, a stamp or a scanned image is not read; there is no OCR.
- **Vertical writing.** Rotated text is read along its baseline; vertical
  writing mode is not.

## Metadata

`metadata "Title" is "Kundenvereinbarung"` reads the Info dictionary
(`Title`, `Author`, `Subject`, `Keywords`, `Creator`, `Producer`,
`CreationDate`, `ModDate`), and where the Info dictionary lacks an entry,
the XMP metadata: `dc:title`, `dc:creator`, `dc:description`,
`pdf:Keywords`, `xmp:CreatorTool`, `pdf:Producer`. PDF 2.0 and PDF/UA
documents carry their title there. Keys are matched regardless of case.

`language is "de-AT"` reads the catalog's `Lang` entry, a BCP 47 tag,
regardless of case. `outline contains "7 Zinsen"` matches the title of an
entry of the document outline (the bookmarks a viewer shows as a table of
contents) at any depth, exactly, after trimming whitespace; a failure lists
the entries there are.

## Accessibility

`is tagged` passes when the catalog's `MarkInfo` says `Marked` and a
structure tree (`StructTreeRoot`) is present. That is necessary for an
accessible document, not sufficient: `conforms to "PDF/UA-1"` has veraPDF
check every rule of the standard, and fails with the rules the document
breaks:

```
the document from "app-client" response body does not conform to PDF/UA-1: 3 rules failed (PDF/UA-1 validation profile)
  7.1-3 (22 checks): Content shall be marked as Artifact or tagged as real content
      at root/document/pages[0]/contentStream[0]/content[0]/contentItem[0]
  7.21.4.1-1 (2 checks): The font programs for all fonts used for rendering within a conforming file shall be embedded within that file
      at root/document/pages[0]/contentStream[0]/operators[10]/font[0] Helvetica-Bold
  7.1-10 (1 check): The document catalog dictionary shall include a ViewerPreferences dictionary containing a DisplayDocTitle key, whose value shall be true
      at root
```

The levels it checks, and veraPDF's own id for each, which the step also
accepts:

| Level | Id | Standard |
|-------|----|----------|
| `PDF/UA-1`, `PDF/UA-2` | `ua1`, `ua2` | ISO 14289, accessibility |
| `PDF/A-1a`, `PDF/A-1b` | `1a`, `1b` | ISO 19005-1 |
| `PDF/A-2a`, `PDF/A-2b`, `PDF/A-2u` | `2a`, `2b`, `2u` | ISO 19005-2 |
| `PDF/A-3a`, `PDF/A-3b`, `PDF/A-3u` | `3a`, `3b`, `3u` | ISO 19005-3 |
| `PDF/A-4`, `PDF/A-4e`, `PDF/A-4f` | `4`, `4e`, `4f` | ISO 19005-4 |
| `WTPDF-1.0-Reuse`, `WTPDF-1.0-Accessibility` | `wt1r`, `wt1a` | Well-Tagged PDF |

## The verapdf preset

```yaml
containers:
  verapdf:
    preset: verapdf
```

The preset runs `verapdf/rest` (a pinned version), the veraPDF consortium's
REST service, and waits until `/api/info` answers on port `8080`. The
conformance step posts the loaded document to it and reads the report back,
so nothing is written to disk and a remote Docker host works as well as a
local one.

The image is published for amd64 only. On an arm64 machine Docker runs it
under emulation: it works, and the JVM takes longer to start, which the
preset's three-minute wait allows for. To run another build, set `image:`;
anything serving the same REST API on port 8080 fits. `env` on the entry
reaches the JVM: `JAVA_OPTS: "-Xmx512m"` for a larger heap.

## Between scenarios

Reset drops the loaded document. The validator container stays up, and a
scenario that asserts on a document reads it first.
