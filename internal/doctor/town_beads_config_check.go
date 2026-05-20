package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/steveyegge/gastown/internal/beads"
)

// TownBeadsConfigCheck verifies town-level .beads/config.yaml exists when
// town beads are enabled.
type TownBeadsConfigCheck struct {
	FixableCheck
	needsRepair bool
}

// NewTownBeadsConfigCheck creates a town-level beads config check.
func NewTownBeadsConfigCheck() *TownBeadsConfigCheck {
	return &TownBeadsConfigCheck{
		FixableCheck: FixableCheck{
			BaseCheck: BaseCheck{
				CheckName:        "town-beads-config",
				CheckDescription: "Verify town .beads/config.yaml exists when beads are enabled",
				CheckCategory:    CategoryConfig,
			},
		},
	}
}

// Run checks if town-level config.yaml exists when town .beads exists.
func (c *TownBeadsConfigCheck) Run(ctx *CheckContext) *CheckResult {
	c.needsRepair = false

	beadsDir := filepath.Join(ctx.TownRoot, ".beads")
	if _, err := os.Stat(beadsDir); os.IsNotExist(err) {
		return &CheckResult{
			Name:     c.Name(),
			Status:   StatusOK,
			Message:  "No town .beads directory (beads not configured)",
			Category: c.CheckCategory,
		}
	} else if err != nil {
		return &CheckResult{
			Name:     c.Name(),
			Status:   StatusWarning,
			Message:  fmt.Sprintf("Could not access town .beads directory: %v", err),
			Category: c.CheckCategory,
		}
	}

	configPath := filepath.Join(beadsDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		c.needsRepair = true
		return &CheckResult{
			Name:     c.Name(),
			Status:   StatusError,
			Message:  "Missing town .beads/config.yaml",
			Details:  []string{fmt.Sprintf("Config: %s", configPath), beadsMetadataDetails(beadsDir), "Fix will create config.yaml without modifying existing metadata or configs."},
			FixHint:  "Run 'gt doctor --fix' to create config.yaml",
			Category: c.CheckCategory,
		}
	} else if err != nil {
		return &CheckResult{
			Name:     c.Name(),
			Status:   StatusWarning,
			Message:  fmt.Sprintf("Could not read town .beads/config.yaml: %v", err),
			Category: c.CheckCategory,
		}
	}

	if data, err := os.ReadFile(configPath); err == nil && !beads.ConfigYAMLDisablesAutoExport(string(data)) {
		c.needsRepair = true
		return &CheckResult{
			Name:     c.Name(),
			Status:   StatusWarning,
			Message:  "Town beads config.yaml must disable export.auto",
			Details:  []string{fmt.Sprintf("Config: %s", configPath), beadsMetadataDetails(beadsDir), "Fix will set export.auto: \"false\" to prevent non-actionable bd auto-export git-add warnings in server-mode runtime beads dirs."},
			FixHint:  "Run 'gt doctor --fix' to repair config.yaml",
			Category: c.CheckCategory,
		}
	} else if err != nil {
		return &CheckResult{
			Name:     c.Name(),
			Status:   StatusWarning,
			Message:  fmt.Sprintf("Could not read town .beads/config.yaml: %v", err),
			Category: c.CheckCategory,
		}
	}

	return &CheckResult{
		Name:     c.Name(),
		Status:   StatusOK,
		Message:  "Town beads config.yaml present",
		Category: c.CheckCategory,
	}
}

// Fix creates or repairs town-level .beads/config.yaml.
func (c *TownBeadsConfigCheck) Fix(ctx *CheckContext) error {
	if !c.needsRepair {
		return nil
	}
	beadsDir := filepath.Join(ctx.TownRoot, ".beads")
	prefix := beads.ConfigDefaultsFromMetadata(beadsDir, "hq")
	return beads.EnsureConfigYAML(beadsDir, prefix)
}

func beadsMetadataDetails(beadsDir string) string {
	metadataPath := filepath.Join(beadsDir, "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return fmt.Sprintf("Metadata: %s unavailable: %v", metadataPath, err)
	}

	var metadata struct {
		Backend      string `json:"backend"`
		DoltMode     string `json:"dolt_mode"`
		DoltDatabase string `json:"dolt_database"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Sprintf("Metadata: %s invalid: %v", metadataPath, err)
	}

	var parts []string
	if metadata.Backend != "" {
		parts = append(parts, "backend="+metadata.Backend)
	}
	if metadata.DoltMode != "" {
		parts = append(parts, "dolt_mode="+metadata.DoltMode)
	}
	if metadata.DoltDatabase != "" {
		parts = append(parts, "dolt_database="+metadata.DoltDatabase)
	}
	if len(parts) == 0 {
		return fmt.Sprintf("Metadata: %s has no backend/dolt fields", metadataPath)
	}

	return fmt.Sprintf("Metadata: %s (%s)", metadataPath, strings.Join(parts, ", "))
}
