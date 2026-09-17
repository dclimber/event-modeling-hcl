# Event Modeling methodology

Use this reference to turn incomplete domain input into a coherent business-time
model before encoding HCL.

## Purpose

Event Modeling creates a shared language for business stakeholders and
engineers. The model tells the system's story from left to right while remaining
close enough to observable behavior to implement and test. It is
technology-neutral: Event Modeling does not require Event Sourcing.

## Five elements

- **Event:** a concrete fact that already happened. Name it in past tense. If it
  can still be rejected, it is probably not an Event.
- **Command:** intent or a request to change state. It may succeed or fail.
- **Read Model:** facts arranged to answer one precise question. No question,
  no Read Model.
- **Screen:** a rough human interaction surface through which an actor observes
  information or initiates behavior. Model information and behavior, not UI
  polish.
- **Automation/gear:** a machine reaction. In HCL the gear is a `processor`; an
  `automation` is a complete top-level workflow pattern.

## Four patterns

| Pattern | Business question | Causal shape |
| --- | --- | --- |
| State Change | What does someone want to change? | Screen/API -> Command -> Event |
| State View | What does someone need to know? | Event(s) -> Read Model -> Screen |
| Automation | What should the system do next by itself? | Internal Event(s) -> Read Model/Processor -> Command -> Event |
| Translation | How is another context's fact translated into our language? | External Event(s) -> Read Model/Processor -> Command -> internal Event |

## Discovery sequence

Use a large group for the first discovery stages, then smaller focused groups
for detailed slices:

1. **Introduction and scope:** establish the modeled system and facilitator.
2. **Brainstorm Events:** collect all relevant facts without organizing them.
3. **Find the plot:** order Events in business time; identify the first Event,
   last Event, and what happens next.
4. **Group Events:** form provisional clusters around capabilities. These may
   change later.
5. **Add first Screens:** make actor-visible parts concrete without designing a
   finished UI.
6. **Add Commands, Read Models, and Automations:** explain how information enters,
   leaves, and causes machine reactions.
7. **Define swimlanes and boundaries:** assign ownership to actors, teams,
   systems, and bounded contexts; mark external boundaries.
8. **Model slice by slice in detail:** add contracts, fields, scenarios, and
   business rules until another engineer could implement without guessing.

Protect discovery from framework, persistence, broker, endpoint, and class
arguments unless they change business behavior.

## Slice design

A slice is one independently useful and testable business capability. Prefer
one Command and one intent per State Change slice. A Command may emit multiple
Events, but this should be exceptional and must remain within one consistency
boundary.

Most multi-step processes do not require atomic consistency. Model sequential,
eventually consistent effects as separate slices connected by Events rather
than one Command touching multiple aggregates.

Use dedicated Read Models by user question or UI component. Reuse only when the
question and concept are genuinely the same. More input Events increase
coupling; investigate whether a Read Model is answering several questions.

Treat external systems as black boxes unless their internal information flow is
itself in scope. Model the facts and data crossing the boundary. A third-party
API call is a Processor/infrastructure action, not automatically a Command in
the modeled system. The internal Command should state the receiving domain's
intent, often storing or acting on the trusted external result.

## Information completeness check

For every attribute on every element, identify the connected source that
provides it or the explicit business rule that generates it.

- A Command contains all information needed for pure business decision logic.
  Trusted current data may be gathered above the handler and included in the
  Command; do not hide external dependencies inside domain decision logic.
- Every Event field comes from the accepted Command or a documented domain
  derivation.
- Every Read Model field comes from its linked Events.
- Every Processor-emitted Command field comes from the consumed Event/Read Model
  or a documented external result.
- Every Screen input needed by its Command is visible and traceable.

A missing path is not a detail to fill creatively. It is an assumption to
resolve or preserve as a hotspot.

## Scenarios and testing semantics

- State Change uses Given Events / When Command / Then Events or Error.
- State View uses Given Events / Then Read Model or Error; there is no When.
- Automation and Translation use Given Event/Read Model / When
  Processor or Command / Then Events or Error.

Scenarios capture business rules, not test plumbing. Start with the happy path,
then add meaningful rejection, boundary, precedence, and transition cases from
the supplied requirements.

## Modeling smells

- **Left chair:** one Command -> many unrelated Events. Likely several intents.
- **Right chair:** many Events -> one Read Model. Likely several questions or
  excessive coupling.
- **Bed:** one Screen -> many unrelated Commands. UI layout may be defining the
  behavior incorrectly.
- **Shelf:** many scenarios on one slice while other important slices have none.
  Business rules remain implicit elsewhere.

These are prompts for discussion, not automatic syntax failures.

## Useful review questions

- Does the model read as a business story from left to right?
- Is every Event an owned past-tense fact?
- Does every workflow represent one capability and one of the four patterns?
- Does every Command have a reason and enough trusted information to decide?
- Does every Read Model answer one precise question?
- Is every field's provenance visible?
- Are external facts translated rather than leaked into the internal language?
- Are unknowns explicit hotspots rather than hidden assumptions?
- Are the scenarios sufficient for implementation without guesswork?
