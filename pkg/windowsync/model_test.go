package windowsync

import (
	"errors"
	"testing"
)

func TestFilterAllowsEveryCodeWhenIncludeIsEmptyExceptExcluded(t *testing.T) {
	filter := FilterConfig{Exclude: []string{" KeyB "}}

	if err := filter.Validate(); err != nil {
		t.Fatal(err)
	}
	if !filter.Allows("KeyA") {
		t.Fatal("expected an unexcluded key to be allowed")
	}
	if filter.Allows("keyb") {
		t.Fatal("expected an excluded key to be rejected")
	}
}

func TestFilterUsesIncludeAsAllowlist(t *testing.T) {
	filter := FilterConfig{Include: []string{"KeyA"}, Exclude: []string{"Space"}}

	if err := filter.Validate(); err != nil {
		t.Fatal(err)
	}
	if !filter.Allows("keya") {
		t.Fatal("expected an included key to be allowed")
	}
	if filter.Allows("Enter") {
		t.Fatal("expected a key outside the include list to be rejected")
	}
	if (FilterConfig{Include: []string{"Space"}, Exclude: []string{"Space"}}).Allows("space") {
		t.Fatal("expected exclude to have priority over include")
	}
}

func TestFilterRejectsDuplicateAndConflictingRules(t *testing.T) {
	tests := []struct {
		name   string
		config FilterConfig
		want   error
	}{
		{name: "duplicate include", config: FilterConfig{Include: []string{"KeyA", "keya"}}, want: ErrDuplicateRule},
		{name: "duplicate exclude", config: FilterConfig{Exclude: []string{"Space", " space "}}, want: ErrDuplicateRule},
		{name: "include exclude conflict", config: FilterConfig{Include: []string{"KeyA"}, Exclude: []string{"keya"}}, want: ErrConflictingRule},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.config.Validate(); !errors.Is(err, test.want) {
				t.Fatalf("Validate() error = %v; want %v", err, test.want)
			}
		})
	}
}

func TestFilterRejectsBlankRules(t *testing.T) {
	if err := (FilterConfig{Include: []string{"  "}}).Validate(); !errors.Is(err, ErrInvalidRule) {
		t.Fatalf("Validate() error = %v; want ErrInvalidRule", err)
	}
}
