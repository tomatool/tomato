# Vendored fonts

`tomato ui` must work offline, so the Geist families the design system calls for
are vendored here instead of loaded from Google Fonts.

| File | Family | Source |
| --- | --- | --- |
| `geist-latin.woff2` | Geist (variable, 400–600) | `@fontsource-variable/geist`, latin subset |
| `geist-mono-latin.woff2` | Geist Mono (variable, 400–600) | `@fontsource-variable/geist-mono`, latin subset |

Geist and Geist Mono are licensed under the SIL Open Font License 1.1.

To refresh:

    curl -sfL https://cdn.jsdelivr.net/npm/@fontsource-variable/geist@latest/files/geist-latin-wght-normal.woff2 -o geist-latin.woff2
    curl -sfL https://cdn.jsdelivr.net/npm/@fontsource-variable/geist-mono@latest/files/geist-mono-latin-wght-normal.woff2 -o geist-mono-latin.woff2

`styles.css` declares both with `font-display:swap` and a system fallback stack,
so a missing file degrades to the system UI/mono font rather than breaking.
