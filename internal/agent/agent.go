package agent

import _ "embed"

//go:embed guide.md
var Guide string

//go:embed short.md
var ShortGuide string

const skillHeader = `---
name: Ink
description: Ink language reference, canonical syntax, diagnostics and fixes for .ink code
---
`

var Skill = skillHeader + Guide
