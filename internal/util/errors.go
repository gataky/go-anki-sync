package util

import "fmt"

// FormatMultipleErrors formats errors into human-readable message.
// Shows up to maxDisplay errors, then truncates with count.
func FormatMultipleErrors(errors []error, maxDisplay int) string {
	if len(errors) == 0 {
		return ""
	}
	if len(errors) == 1 {
		return errors[0].Error()
	}

	msg := fmt.Sprintf("%d cards failed: ", len(errors))
	for i, err := range errors {
		if i > 0 {
			msg += "; "
		}
		msg += err.Error()
		if i >= maxDisplay-1 && len(errors) > maxDisplay {
			msg += fmt.Sprintf("; and %d more", len(errors)-maxDisplay)
			break
		}
	}
	return msg
}
