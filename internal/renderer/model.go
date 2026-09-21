// Package renderer turns a validated Event Modeling IR into a self-contained
// interactive HTML canvas.
package renderer

import (
	"encoding/json"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/version"
)

const specVersion = version.Spec

type ViewModel struct {
	Title      string             `json:"title"`
	Version    string             `json:"version"`
	Actors     map[string]Actor   `json:"actors"`
	Contexts   map[string]Context `json:"contexts"`
	ContextMap ContextMap         `json:"contextMap"`
	Chapters   []Chapter          `json:"chapters"`
	Hotspots   []Hotspot          `json:"hotspots"`
	Slices     []Slice            `json:"slices"`
	Edges      []Edge             `json:"edges"`
}

type Actor struct {
	Title        string `json:"title"`
	AuthRequired bool   `json:"authRequired"`
}

type Context struct {
	Title    string `json:"title"`
	External bool   `json:"external"`
}

type ContextMap struct {
	Nodes []ContextNode `json:"nodes"`
	Edges []ContextEdge `json:"edges"`
}

type ContextNode struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	External   bool   `json:"external,omitempty"`
	Team       string `json:"team,omitempty"`
	Events     int    `json:"events"`
	Aggregates int    `json:"aggregates"`
}

type ContextEdge struct {
	Upstream   string   `json:"upstream"`
	Downstream string   `json:"downstream"`
	Pattern    string   `json:"pattern"`
	Via        []string `json:"via,omitempty"`
}

type Chapter struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Slices []string `json:"slices"`
}

type Hotspot struct {
	ID       string `json:"id"`
	OnID     string `json:"onId,omitempty"`
	Target   string `json:"target,omitempty"`
	Status   string `json:"status"`
	Question string `json:"question"`
}

type Slice struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Title       string     `json:"title"`
	Status      string     `json:"status,omitempty"`
	Owner       string     `json:"owner,omitempty"`
	Description string     `json:"description,omitempty"`
	StageCount  int        `json:"stageCount"`
	Elements    []Element  `json:"elements"`
	Scenarios   []Scenario `json:"scenarios"`
}

type Element struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Title    string   `json:"title"`
	Stage    int      `json:"stage"`
	Actor    string   `json:"actor,omitempty"`
	ImageURL string   `json:"imageUrl,omitempty"`
	Agg      string   `json:"agg,omitempty"`
	Ctx      string   `json:"ctx,omitempty"`
	External bool     `json:"external,omitempty"`
	Given    bool     `json:"given,omitempty"`
	API      string   `json:"api,omitempty"`
	Question string   `json:"question,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Fields   []Field  `json:"fields,omitempty"`
}

type Field struct {
	Name string `json:"name"`
	Type string `json:"type"`
	ID   bool   `json:"id,omitempty"`
	PII  bool   `json:"pii,omitempty"`
}

type Scenario struct {
	Title    string   `json:"title"`
	Given    []Step   `json:"given,omitempty"`
	When     *Step    `json:"when,omitempty"`
	Then     []Step   `json:"then,omitempty"`
	Comments []string `json:"comments,omitempty"`
}

type Step struct {
	Title     string          `json:"title"`
	RefKind   string          `json:"refKind,omitempty"`
	Ref       string          `json:"ref,omitempty"`
	Examples  json.RawMessage `json:"examples,omitempty"`
	Fields    []string        `json:"fields,omitempty"`
	Error     string          `json:"error,omitempty"`
	EmptyList bool            `json:"emptyList,omitempty"`
}

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}
