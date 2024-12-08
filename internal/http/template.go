package http

import "strings"

type templateData struct {
	data map[string]any
}

func newTemplateData(contextRoot string) *templateData {
	td := &templateData{
		data: make(map[string]any),
	}
	return td.withContextRoot(contextRoot)
}

func (t *templateData) withContextRoot(contextRoot string) *templateData {
	if !strings.HasSuffix(contextRoot, "/") {
		t.data["ContextRoot"] = contextRoot + "/"
	} else {
		t.data["ContextRoot"] = contextRoot
	}

	return t
}

func (t *templateData) withScripts(scripts ...string) *templateData {
	t.data["Scripts"] = scripts
	return t
}
func (t *templateData) withStylesheets(stylesheets ...string) *templateData {
	t.data["Stylesheets"] = stylesheets
	return t
}

func (t *templateData) withBodyClass(bodyClass string) *templateData {
	t.data["BodyClass"] = bodyClass
	return t
}

func (t *templateData) hasError() bool {
	_, hasError := t.data["Error"]
	return hasError
}

func (t *templateData) addError(elementName string, errorText string) {

}
