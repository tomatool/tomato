# Browser

Steps for using a web UI in a real Chromium, the way a person does: by what the page shows

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Navigation

| Step | Description |
|------|-------------|
| `"alice-browser" opens "/master"` | Open a page and wait until it has loaded. A path is joined to base_url |



## Using the Page

| Step | Description |
|------|-------------|
| `"alice-browser" clicks "Apply"` | Click what is called this: a button, link or tab by its accessible name, or else the element showing this text. Waits until it is there, enabled and not covered |
| `"alice-browser" fills "Search" with "Zinsen"` | Replace what a field holds. The field is named by its label, placeholder or accessible name; a contenteditable editor counts as a field |
| `"alice-browser" types " und Gebühren" at the end of "7.2"` | Put the caret at the end of the paragraph or field showing this text, and type there key by key |
| `"alice-browser" types "Neuer Satz."` | Type key by key into whatever has focus |
| `"alice-browser" presses "ctrl+z"` | Press a key or a shortcut, such as Enter, Escape, Tab, ArrowDown or ctrl+z |
| `"alice-browser" checks "Austria"` | Tick a checkbox or switch by its label, unless it is ticked already |
| `"alice-browser" unchecks "Netherlands"` | Clear a checkbox or switch by its label, unless it is clear already |
| `"alice-browser" chooses "Selected countries"` | Choose a radio button, tab or option by its label |
| `"alice-browser" selects "Austria" from "Country"` | Pick an option of a select box or combobox, both by what they show |



## What the Page Shows

| Step | Description |
|------|-------------|
| `"alice-browser" sees "7 Zinsen"` | Assert the page shows this text now, in its content or in a field |
| `"bob-browser" sees "Neuer Satz." within "3s"` | Wait until the page shows this text, for at most the duration |
| `"alice-browser" does not see "clause_id"` | Assert the page does not show this text now |
| `"alice-browser" no longer sees "Saving…" within "5s"` | Wait until the page stops showing this text, for at most the duration |
| `"alice-browser" sees "Apply" is disabled` | Assert a button, field or other control is disabled |
| `"alice-browser" sees "Apply" is enabled` | Assert a button, field or other control is enabled |
| `"alice-browser" url is "/master/7.2"` | Assert the page's address: the path with its query and fragment, or the whole URL when one is given |



## Screenshots

| Step | Description |
|------|-------------|
| `"alice-browser" saves screenshot "after-apply"` | Save what the page shows as <name>.png in the screenshots directory |


