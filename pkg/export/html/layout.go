package html

// component names how a region's content is laid out in its panel.
type component int

const (
	// compText renders the region's markdown as it is.
	compText component = iota
	// compGoals pairs goals and non-goals as an ok and a neutral card.
	compGoals
	// compScope pairs in-scope and out-of-scope as an ok and a bad card.
	compScope
	// compChecklist renders the region's list as open tasks.
	compChecklist
	// compLastVerified is a card around the Last Verified table.
	compLastVerified
	// compDecision opens with the decision's first block in a callout.
	compDecision
	// compConsequences is three cards: positive, negative, neutral.
	compConsequences
	// compAlternatives is one card per alternative.
	compAlternatives
	// compRisks tints the risks table's likelihood and impact cells.
	compRisks
	// compCriteria is one ok card per criterion.
	compCriteria
	// compPhases is the filter bar, then one phase card per phase.
	compPhases
	// compFindings is one card per finding.
	compFindings
	// compVerdict opens with the **Answer:** line as a verdict.
	compVerdict
	// compProcedures is a runbook's procedures, steps numbered.
	compProcedures
	// compScenarios is one expandable per runbook scenario.
	compScenarios
	// compQuestions is one expandable per open question.
	compQuestions
)

// The kinds and the tab this package names more than once.
const (
	kindSummary      = "summary"
	kindScope        = "scope"
	kindConsequences = "consequences"
	kindProcedure    = "procedure"
	kindPhase        = "phase"
	tabOverview      = "Overview"
)

// row is one region kind's place on the page.
type row struct {
	// Tab is the label of the tab the region goes to.
	Tab string
	// Component lays the region out.
	Component component
}

// layout is DESIGN-0025 §4's table: every region kind docz lays out, the
// tab it goes to, and how. Tabs appear in the order their first region
// does, so the table carries no order of its own. A kind it does not list
// gets a tab of its own named by its heading, laid out as text.
var layout = map[string]row{
	kindSummary:  {tabOverview, compText},
	"overview":   {tabOverview, compText},
	"objective":  {tabOverview, compText},
	"question":   {tabOverview, compText},
	"hypothesis": {tabOverview, compText},
	"context":    {tabOverview, compText},
	"goals":      {tabOverview, compGoals},
	"non-goals":  {tabOverview, compGoals},
	kindScope:    {tabOverview, compScope},
	"when":       {tabOverview, compText},

	"prerequisites": {tabOverview, compChecklist},
	"last-verified": {tabOverview, compLastVerified},

	"problem":         {"Problem", compText},
	"background":      {"Background", compText},
	"proposal":        {"Proposal", compText},
	"detailed-design": {"Design", compText},
	"decision":        {"Decision", compDecision},
	kindConsequences:  {"Consequences", compConsequences},
	"alternatives":    {"Alternatives", compAlternatives},
	"risks":           {"Risks", compRisks},
	"criteria":        {"Success", compCriteria},
	"api-changes":     {"Interfaces", compText},
	"data-model":      {"Data model", compText},
	kindPhase:         {"Phases", compPhases},
	"file-changes":    {"Changes", compText},
	"testing":         {"Testing", compText},
	"rollout":         {"Rollout", compText},
	"dependencies":    {"Dependencies", compText},
	"approach":        {"Approach", compText},
	"environment":     {"Approach", compText},
	"findings":        {"Findings", compFindings},
	"conclusion":      {"Conclusion", compVerdict},
	"recommendation":  {"Conclusion", compText},
	kindProcedure:     {"Procedures", compProcedures},
	"scenario":        {"Scenarios", compScenarios},
	"escalation":      {"Escalation", compText},
	"open-questions":  {"Open questions", compQuestions},
	"decisions":       {"Decisions", compText},
	"references":      {"References", compText},
}

// rowFor is a kind's row, and false for a kind the table does not list.
func rowFor(kind string) (row, bool) {
	r, ok := layout[kind]

	return r, ok
}
