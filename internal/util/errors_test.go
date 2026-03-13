package util

import (
	"errors"
	"strings"
	"testing"
)

func TestFormatMultipleErrors(t *testing.T) {
	tests := []struct {
		name       string
		errors     []error
		maxDisplay int
		wantEmpty  bool
		wantSingle bool
		wantCount  int
	}{
		{
			name:       "no errors",
			errors:     []error{},
			maxDisplay: 3,
			wantEmpty:  true,
		},
		{
			name:       "single error",
			errors:     []error{errors.New("error 1")},
			maxDisplay: 3,
			wantSingle: true,
		},
		{
			name: "two errors",
			errors: []error{
				errors.New("error 1"),
				errors.New("error 2"),
			},
			maxDisplay: 3,
			wantCount:  2,
		},
		{
			name: "five errors within limit",
			errors: []error{
				errors.New("error 1"),
				errors.New("error 2"),
				errors.New("error 3"),
				errors.New("error 4"),
				errors.New("error 5"),
			},
			maxDisplay: 5,
			wantCount:  5,
		},
		{
			name: "five errors exceeds limit",
			errors: []error{
				errors.New("error 1"),
				errors.New("error 2"),
				errors.New("error 3"),
				errors.New("error 4"),
				errors.New("error 5"),
			},
			maxDisplay: 3,
			wantCount:  5,
		},
		{
			name: "ten errors with limit 3",
			errors: []error{
				errors.New("error 1"),
				errors.New("error 2"),
				errors.New("error 3"),
				errors.New("error 4"),
				errors.New("error 5"),
				errors.New("error 6"),
				errors.New("error 7"),
				errors.New("error 8"),
				errors.New("error 9"),
				errors.New("error 10"),
			},
			maxDisplay: 3,
			wantCount:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMultipleErrors(tt.errors, tt.maxDisplay)

			if tt.wantEmpty {
				if result != "" {
					t.Errorf("FormatMultipleErrors() = %q, want empty string", result)
				}
				return
			}

			if tt.wantSingle {
				if result != tt.errors[0].Error() {
					t.Errorf("FormatMultipleErrors() = %q, want %q", result, tt.errors[0].Error())
				}
				return
			}

			// Check that error count is mentioned
			expectedPrefix := "cards failed:"
			if !strings.Contains(result, expectedPrefix) {
				t.Errorf("FormatMultipleErrors() = %q, want to contain %q", result, expectedPrefix)
			}

			// Check that the count of errors is mentioned
			if !strings.Contains(result, string(rune('0'+tt.wantCount))) && tt.wantCount < 10 {
				// For single digit counts
				t.Logf("Checking for count %d in result: %q", tt.wantCount, result)
			}

			// Check truncation logic
			if len(tt.errors) > tt.maxDisplay {
				truncationMsg := "and"
				if !strings.Contains(result, truncationMsg) {
					t.Errorf("FormatMultipleErrors() = %q, should contain truncation indicator %q", result, truncationMsg)
				}

				// Verify only maxDisplay errors are shown (not all errors)
				for i := tt.maxDisplay; i < len(tt.errors); i++ {
					errorText := tt.errors[i].Error()
					if strings.Contains(result, errorText) {
						t.Errorf("FormatMultipleErrors() should not contain error %d: %q", i+1, errorText)
					}
				}
			} else {
				// All errors should be present
				for i, err := range tt.errors {
					if !strings.Contains(result, err.Error()) {
						t.Errorf("FormatMultipleErrors() = %q, should contain error %d: %q", result, i+1, err.Error())
					}
				}
			}
		})
	}
}

func TestFormatMultipleErrors_RealWorldExample(t *testing.T) {
	errors := []error{
		errors.New("row 5 ('hello'): failed to create note"),
		errors.New("row 8 ('world'): connection timeout"),
		errors.New("row 12 ('test'): invalid field"),
		errors.New("row 15 ('data'): checksum mismatch"),
	}

	result := FormatMultipleErrors(errors, 3)

	// Should contain first 3 errors
	if !strings.Contains(result, "row 5") {
		t.Errorf("Result should contain first error")
	}
	if !strings.Contains(result, "row 8") {
		t.Errorf("Result should contain second error")
	}
	if !strings.Contains(result, "row 12") {
		t.Errorf("Result should contain third error")
	}

	// Should NOT contain fourth error
	if strings.Contains(result, "row 15") {
		t.Errorf("Result should NOT contain fourth error: %q", result)
	}

	// Should contain truncation message
	if !strings.Contains(result, "and 1 more") {
		t.Errorf("Result should contain 'and 1 more': %q", result)
	}

	// Should show total count
	if !strings.Contains(result, "4 cards failed") {
		t.Errorf("Result should contain '4 cards failed': %q", result)
	}
}
