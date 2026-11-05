package sim

import "flag"

// Development flags let one seed or policy be printed without dumping thirty of
// them into the terminal.
var (
	seedFlag   = flag.String("seed", "", "seed to inspect")
	policyFlag = flag.String("policy", "", "policy to inspect")
)
