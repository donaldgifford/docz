package html

import (
	"regexp"
	"strings"
	"testing"
)

func TestRenderOpenQuestions(t *testing.T) {
	t.Parallel()

	p := page(t, "<!--docz:open-questions:start-->\n## Open Questions\n\nTwo remain.\n\n"+
		"### 1. Which store?\n\n- a. Postgres *(recommendation)*\n- b. SQLite\n\n"+
		"> **Resolved 2026-10-31: (a).** Postgres it is.\n>\n> **Revised 2026-11-01:** still Postgres.\n\n"+
		"### 2. Which queue?\n\n- a. Redis\n- b. NATS *(recommended)*\n\n"+
		"<!--docz:open-questions:end-->\n", &RenderOptions{})
	body := string(p.Body)

	for _, want := range []string{
		`<p>Two remain.</p>`,
		`<details class="expand" id="1-which-store">`,
		`<details class="expand" id="2-which-queue" open>`,
		`<li class="option recommended">a. Postgres`,
		`<li class="option">b. SQLite</li>`,
		`<li class="option">a. Redis</li>`,
		`<li class="option recommended">b. NATS`,
		`<p class="resolution"><strong>Resolved 2026-10-31: (a).</strong> Postgres it is.</p>`,
		`<p class="resolution"><strong>Revised 2026-11-01:</strong> still Postgres.</p>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %s", want)
		}
	}

	if n := len(regexp.MustCompile(`<details class="expand"`).FindAllString(body, -1)); n != 2 {
		t.Errorf("%d expandables, want 2", n)
	}
}
