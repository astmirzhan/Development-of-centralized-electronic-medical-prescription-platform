// Package seed embeds synthetic reference datasets for the CLI seeder.
package seed

import "embed"

// DataFS holds embedded JSON datasets for ICD-10, INN medicines, and DDI pairs.
//
//go:embed data/*.json
var DataFS embed.FS
