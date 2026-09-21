package renderer

import (
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/model"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/version"
)

//go:embed assets/viewer.shell.html
var viewerShell string

//go:embed assets/viewer.css
var viewerCSS string

//go:embed assets/viewer.js
var viewerJS string

//go:embed assets/viewer.eventstorming.css
var viewerEventStormingCSS string

//go:embed assets/viewer.contextmap.css
var viewerContextMapCSS string

//go:embed assets/viewer.eventstorming.js
var viewerEventStormingJS string

//go:embed assets/viewer.contextmap.js
var viewerContextMapJS string

const (
	stylesMarker      = "__STYLES__"
	stylesESMarker    = "__STYLES_ES__"
	stylesCMMarker    = "__STYLES_CM__"
	scriptMarker      = "__SCRIPT__"
	scriptESMarker    = "__SCRIPT_ES__"
	scriptCMMarker    = "__SCRIPT_CM__"
	modelMarker       = "__MODEL_JSON__"
	specVersionMarker = "__SPEC_VERSION__"
)

func RenderHTML(view *ViewModel) (string, error) {
	encoded, err := json.Marshal(view)
	if err != nil {
		return "", err
	}
	doc := strings.Replace(viewerShell, stylesMarker, viewerCSS, 1)
	doc = strings.Replace(doc, stylesESMarker, viewerEventStormingCSS, 1)
	doc = strings.Replace(doc, stylesCMMarker, viewerContextMapCSS, 1)
	doc = strings.Replace(doc, scriptMarker, viewerJS, 1)
	doc = strings.Replace(doc, scriptESMarker, viewerEventStormingJS, 1)
	doc = strings.Replace(doc, scriptCMMarker, viewerContextMapJS, 1)
	doc = strings.Replace(doc, specVersionMarker, version.Spec, 1)
	return strings.Replace(doc, modelMarker, string(encoded), 1), nil
}

func Render(filename string, source *model.Model) (string, error) {
	return RenderHTML(BuildViewModel(filename, source))
}
