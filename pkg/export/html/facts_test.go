package html

import (
	"strings"
	"testing"
)

func TestRenderTypedParseErrorFallsBack(t *testing.T) {
	t.Parallel()

	proc := func(title, step string) string {
		return "<!--docz:procedure:start-->\n### Procedure 1: " + title + "\n\n" +
			"<!--docz:steps:start-->\n1. " + step + "\n<!--docz:steps:end-->\n<!--docz:procedure:end-->\n\n"
	}

	p := page(t, proc("Deploy", "Run the deploy.")+proc("Roll back", "Run the rollback."), &RenderOptions{})
	body := string(p.Body)

	if len(p.Problems) != 1 || !strings.Contains(p.Problems[0].Error(), "runbook layout") {
		t.Fatalf("Problems = %v, want one runbook layout problem", p.Problems)
	}

	for _, want := range []string{"Run the deploy.", "Run the rollback."} {
		if !strings.Contains(body, want) {
			t.Errorf("the fallback lost %q", want)
		}
	}

	if strings.Contains(body, `<ol class="steps">`) {
		t.Errorf("a procedure whose parse failed is laid out as steps")
	}
}
