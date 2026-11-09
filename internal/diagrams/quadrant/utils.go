// Ported from mermaid 12.2.0, diagrams/quadrant-chart/utils.ts (MIT).

package quadrant

import (
	"fmt"
	"regexp"
)

// invalidStyleError is InvalidStyleError. value is the text of the value,
// or "undefined".
func invalidStyleError(style, value, typ string) error {
	return fmt.Errorf("value for %s %s is invalid, please use a valid %s", style, value, typ)
}

var (
	reHexCode      = regexp.MustCompile(`^#?([\dA-Fa-f]{6}|[\dA-Fa-f]{3})$`)
	reNumber       = regexp.MustCompile(`^\d+$`)
	reSizeInPixels = regexp.MustCompile(`^\d+px$`)
)

// validateHexCode is validateHexCode(value): true for an invalid value.
func validateHexCode(value string) bool { return !reHexCode.MatchString(value) }

// validateNumber is validateNumber(value): true for an invalid value.
func validateNumber(value string) bool { return !reNumber.MatchString(value) }

// validateSizeInPixels is validateSizeInPixels(value): true for an invalid
// value.
func validateSizeInPixels(value string) bool { return !reSizeInPixels.MatchString(value) }
