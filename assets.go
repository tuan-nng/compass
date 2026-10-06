// Package compass holds the files compass embeds at build time: the org
// configuration, the okf pin (plan decision 7) and the OKF skill.
package compass

import "embed"

// ConfigEnv is config.env: OKF_ORG, which an OKF_ORG environment variable
// overrides.
//
//go:embed config.env
var ConfigEnv string

// PinsEnv is pins/okf.env: the pinned okf release and its checksums.
//
//go:embed pins/okf.env
var PinsEnv string

// Skill holds skill/okf/, the OKF skill that `compass setup` installs into
// each agent's user-level skill folder.
//
//go:embed skill/okf
var Skill embed.FS
