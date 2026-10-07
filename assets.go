// Package compass holds the files compass embeds at build time: the okf pin
// (plan decision 5) and the OKF skill.
package compass

import "embed"

// PinsEnv is pins/okf.env: the pinned okf release and its checksums.
//
//go:embed pins/okf.env
var PinsEnv string

// Skill holds skill/okf/, the OKF skill that `compass setup` installs into
// each agent's user-level skill folder.
//
//go:embed skill/okf
var Skill embed.FS
