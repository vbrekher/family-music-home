# Family Music Home design system

A quiet music collection with room for stories. The product uses the user's existing theme; no fixed light surface or dark text is imposed on a theme.

## Layout and type

Use a 4 px base, with 8/16/24/32 px spacing. Pages have 32 px desktop and 16 px mobile gutters, a 1440 px reading boundary, and 96 px clearance above the player. Flex/grid children must have `min-width: 0`. Long music names wrap, rather than enlarge the page. Forms should have a clear single primary action; secondary controls remain visually quieter.

Page titles use a responsive 29–48 px scale, section headings 22 px, body text the inherited accessible theme size. Weight and spacing create hierarchy without excessive uppercase. Artwork belongs beside the music it identifies; it never serves as an unexplained action.

## Components

Family surfaces have a 16 px radius, a theme divider border and restrained depth. Cards use 24 px content spacing. Primary buttons and navigation targets are at least 44 px high; Kids controls remain 48 px. Visible keyboard focus uses a 3 px ring and an offset. Buttons wrap long labels. Form controls have consistent vertical spacing, labels and field errors.

Desktop section navigation wraps within its surface and marks the current page. At widths below 960 px, a labelled native section selector replaces the long horizontal navigation strip. This preserves all destinations without requiring sideways scrolling or tiny controls.

Use theme text, background, divider, selected and primary tokens. Color alone cannot communicate state. Loading, empty, unavailable and permission errors must explain the next action. A failed source remains in the catalog; the interface must never imply that missing network access deleted music.

## Verification

Audit actual running screens at 360, 375, 390, 768, 1024, 1280, 1440 and 1920 CSS px, with English/Russian, long labels, 200% zoom, keyboard use and all 27 themes. A code style rule is not evidence that these checks passed. Record findings and evidence in UI_AUDIT.md.
