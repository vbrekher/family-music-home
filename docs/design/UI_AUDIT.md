# UI audit — Family Music Home 1.0.0

Scope: real local browser acceptance using original synthetic music and fictional accounts. No personal library or production screenshots are published. Project Owner / Maintainer: @aitishnyk. Project Co-author: @krisbrekher.

| Finding | Implemented correction | Observed verification |
|---|---|---|
| Navigation overflow and unclear current section | Wrapping desktop navigation, labelled mobile selector, current-page semantics including nested detail routes | Eight CSS viewport widths; two dedicated nested-route regressions |
| Uneven spacing and small controls | Shared scoped family surfaces, responsive headings, 44 px form controls and focus ring | Browser inspection and keyboard traversal |
| Home customization controls crowd narrow screens | Paired control grid, bounded dialog width, scrollable body and fixed actions | 360/375/390/768/1024/1280/1440/1920 CSS px; zero painted control overlap |
| Reordering did not immediately reflect saved order | Render enabled widgets in their actual draft order | Regression moves a section and checks displayed and submitted order |
| Primary button text and player title unreadable in dark themes | Contrast-based primary labels; normal theme text for player title | Extra Dark and Nord browser inspection; all 27 bundled themes switched and captured |
| Technical passport facts difficult to scan | Responsive fact cards and wrapping long values | Final browser inspection; no sample-rate quality verdict |
| Fork identity and attribution absent | Family Music Home branding and separate project credits, retaining upstream attribution | Source and About review |

## Evidence and reproducibility

[Before](../assets/home-before.png) · [After](../assets/home-after.png) · [Gallery](../en/gallery.md) · [Design system](DESIGN_SYSTEM.md).

The eight viewport checks used a local desktop browser, not physical phones or tablets. The customization dialog measured 328/343/358 px at the three smallest widths and 600 px at larger widths. Document width matched viewport width; clipped, scrolled-out content was excluded from painted overlap measurements. Keyboard Tab reached checkboxes and reorder buttons with a visible focus indicator.

Native 200% browser zoom could not be controlled by the available in-app browser and is **NOT_RUN**. A 720 CSS px viewport checks the reflow width corresponding to a 1440 px viewport at 200%; it is not evidence of native zoom. Physical mobile devices, real NAS hardware and the visible macOS notification banner are **NOT_RUN**. These limits do not imply production deployment acceptance.

All screenshots use generated compositions and artwork. The high-resolution demo edition is a deliberately resampled synthetic signal; format properties do not establish mastering quality. New family screens currently use English labels; inherited Navidrome screens support Russian. Documentation is supplied in English and Russian.
