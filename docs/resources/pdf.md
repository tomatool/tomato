# PDF

Steps for asserting on what the PDF documents your application generates contain: pages, text, metadata, outline and accessibility

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Reading

| Step | Description |
|------|-------------|
| `"{resource}" reads file "out/ca.pdf"` | Read a PDF from a file on disk, relative to where tomato runs |
| `"{resource}" reads "files" object "cas/CA_Version_No._07.07_AT-de.pdf"` | Read a PDF from an object of an s3 resource, as "bucket/key" |
| `"{resource}" reads "api" response body` | Read a PDF from the body of the last response an http resource received |



## Pages

| Step | Description |
|------|-------------|
| `"{resource}" has "142" pages` | Assert the number of pages |



## Text

| Step | Description |
|------|-------------|
| `"{resource}" text contains "9 Kryptowerte"` | Assert the document's text contains this, on any page |
| `"{resource}" text does not contain "{{"` | Assert the document's text does not contain this anywhere, e.g. an unrendered template placeholder |
| `"{resource}" page "3" contains "Inhaltsverzeichnis"` | Assert the text of one page, counted from 1, contains this |
| `"{resource}" text "Conditions générales" appears before "General Terms and Conditions"` | Assert one piece of text comes before another in the document, e.g. the German block of a bilingual contract before the English one |



## Metadata

| Step | Description |
|------|-------------|
| `"{resource}" metadata "Title" is "Kundenvereinbarung"` | Assert a metadata entry (Title, Author, Subject, Producer, Creator, Keywords) from the Info dictionary, or the XMP metadata when the Info dictionary lacks it |
| `"{resource}" language is "de-AT"` | Assert the document's language, the catalog's Lang entry as a BCP 47 tag |
| `"{resource}" outline contains "7 Zinsen"` | Assert the outline (the bookmarks a viewer shows as a table of contents) has an entry with this title, at any level |



## Accessibility

| Step | Description |
|------|-------------|
| `"{resource}" is tagged` | Assert the document is tagged: MarkInfo says Marked and a structure tree is present |
| `"{resource}" conforms to "PDF/UA-1"` | Assert the document conforms to PDF/UA-1, PDF/UA-2 or a PDF/A level (1a, 1b, 2a, 2b, 2u, 3a, 3b, 3u, 4, 4e, 4f), checked by the veraPDF container named in options.validator; fails with the rules it breaks |


