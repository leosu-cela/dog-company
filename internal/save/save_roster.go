package save

// validRosterID is the v3+ Dog roster whitelist (65 entries).
// Source: src/constants/dogRoster.ts on the frontend repo.
// Sync rule: when the roster changes, copy the new id list here verbatim.
var validRosterID = map[string]struct{}{
	// tech (16)
	"tech-D-1": {}, "tech-D-2": {}, "tech-D-3": {}, "tech-D-4": {}, "tech-D-5": {}, "tech-D-6": {},
	"tech-C-1": {}, "tech-C-2": {}, "tech-C-3": {}, "tech-C-4": {},
	"tech-B-1": {}, "tech-B-2": {}, "tech-B-3": {},
	"tech-A-1": {}, "tech-A-2": {},
	"tech-S-1": {},
	// design (16)
	"design-D-1": {}, "design-D-2": {}, "design-D-3": {}, "design-D-4": {}, "design-D-5": {}, "design-D-6": {},
	"design-C-1": {}, "design-C-2": {}, "design-C-3": {}, "design-C-4": {},
	"design-B-1": {}, "design-B-2": {}, "design-B-3": {},
	"design-A-1": {}, "design-A-2": {},
	"design-S-1": {},
	// marketing (16)
	"mkt-D-1": {}, "mkt-D-2": {}, "mkt-D-3": {}, "mkt-D-4": {}, "mkt-D-5": {}, "mkt-D-6": {},
	"mkt-C-1": {}, "mkt-C-2": {}, "mkt-C-3": {}, "mkt-C-4": {},
	"mkt-B-1": {}, "mkt-B-2": {}, "mkt-B-3": {},
	"mkt-A-1": {}, "mkt-A-2": {},
	"mkt-S-1": {},
	// service (16)
	"svc-D-1": {}, "svc-D-2": {}, "svc-D-3": {}, "svc-D-4": {}, "svc-D-5": {}, "svc-D-6": {},
	"svc-C-1": {}, "svc-C-2": {}, "svc-C-3": {}, "svc-C-4": {},
	"svc-B-1": {}, "svc-B-2": {}, "svc-B-3": {},
	"svc-A-1": {}, "svc-A-2": {},
	"svc-S-1": {},
	// U (1)
	"u-1": {},
}
