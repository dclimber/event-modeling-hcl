// Package interchange converts Event Modeling HCL to and from the slice-based
// JSON format defined by the published Event Modeling schema.
package interchange

import "encoding/json"

// Document is the root object of the interchange format.
type Document struct {
	Slices []Slice `json:"slices"`
}

type Slice struct {
	ID             string          `json:"id"`
	Status         string          `json:"status,omitempty"`
	Index          *float64        `json:"index,omitempty"`
	Title          string          `json:"title"`
	Context        string          `json:"context,omitempty"`
	SliceType      string          `json:"sliceType"`
	Assignee       string          `json:"assignee,omitempty"`
	TicketNumber   string          `json:"ticketNumber,omitempty"`
	Commands       []Element       `json:"commands"`
	Events         []Element       `json:"events"`
	ReadModels     []Element       `json:"readmodels"`
	Screens        []Element       `json:"screens"`
	ScreenImages   []ScreenImage   `json:"screenImages,omitempty"`
	Processors     []Element       `json:"processors"`
	Tables         []Table         `json:"tables"`
	Specifications []Specification `json:"specifications"`
	Storylines     []Storyline     `json:"storylines,omitempty"`
	Actors         []Actor         `json:"actors,omitempty"`
	Aggregates     []string        `json:"aggregates,omitempty"`
}

type Element struct {
	GroupID               string          `json:"groupId,omitempty"`
	ID                    string          `json:"id"`
	Tags                  []string        `json:"tags,omitempty"`
	Domain                string          `json:"domain,omitempty"`
	ModelContext          string          `json:"modelContext,omitempty"`
	Context               string          `json:"context,omitempty"`
	Slice                 string          `json:"slice,omitempty"`
	Title                 string          `json:"title"`
	Fields                []Field         `json:"fields"`
	Type                  string          `json:"type"`
	Description           string          `json:"description,omitempty"`
	Aggregate             string          `json:"aggregate,omitempty"`
	AggregateDependencies []string        `json:"aggregateDependencies,omitempty"`
	Dependencies          []Dependency    `json:"dependencies"`
	APIEndpoint           string          `json:"apiEndpoint,omitempty"`
	Service               *string         `json:"service,omitempty"`
	CreatesAggregate      bool            `json:"createsAggregate,omitempty"`
	Triggers              []string        `json:"triggers,omitempty"`
	Sketched              bool            `json:"sketched,omitempty"`
	Prototype             json.RawMessage `json:"prototype,omitempty"`
	ListElement           bool            `json:"listElement,omitempty"`
	LinkedID              string          `json:"linkedId,omitempty"`
	ElementCopy           bool            `json:"elementCopy,omitempty"`
}

type ScreenImage struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
}

type Table struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Fields []Field `json:"fields"`
}

type Specification struct {
	Vertical  bool                `json:"vertical,omitempty"`
	ID        string              `json:"id"`
	SliceName string              `json:"sliceName,omitempty"`
	Title     string              `json:"title"`
	Given     []SpecificationStep `json:"given"`
	When      []SpecificationStep `json:"when"`
	Then      []SpecificationStep `json:"then"`
	Comments  []Comment           `json:"comments,omitempty"`
	LinkedID  string              `json:"linkedId"`
}

type SpecificationStep struct {
	Title           string            `json:"title"`
	Tags            []string          `json:"tags,omitempty"`
	Examples        []json.RawMessage `json:"examples,omitempty"`
	ID              string            `json:"id"`
	Index           *float64          `json:"index,omitempty"`
	SpecRow         *float64          `json:"specRow,omitempty"`
	Type            string            `json:"type"`
	Fields          []Field           `json:"fields,omitempty"`
	LinkedID        string            `json:"linkedId,omitempty"`
	ExpectEmptyList bool              `json:"expectEmptyList,omitempty"`
}

type Comment struct {
	Description string `json:"description"`
}

type Actor struct {
	Name          string   `json:"name"`
	AuthRequired  bool     `json:"authRequired"`
	RolesRequired []string `json:"rolesRequired,omitempty"`
	Tags          []string `json:"tags,omitempty"`
}

type Storyline struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Elements []Element `json:"elements"`
}

type Dependency struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	ElementType string `json:"elementType"`
}

type Field struct {
	Name               string          `json:"name"`
	Type               string          `json:"type"`
	Example            json.RawMessage `json:"example,omitempty"`
	Subfields          []Field         `json:"subfields,omitempty"`
	Mapping            string          `json:"mapping,omitempty"`
	Optional           bool            `json:"optional,omitempty"`
	TechnicalAttribute bool            `json:"technicalAttribute,omitempty"`
	Generated          bool            `json:"generated,omitempty"`
	IDAttribute        bool            `json:"idAttribute,omitempty"`
	PII                bool            `json:"pii,omitempty"`
	Schema             string          `json:"schema,omitempty"`
	Cardinality        string          `json:"cardinality,omitempty"`
}
