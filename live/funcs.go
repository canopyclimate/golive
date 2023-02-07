package live

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/canopyclimate/golive/htmltmpl"
)

// Funcs provides some useful function for live.View templates:
//   - liveTitleTag: renders a title tag that can be updated from Views
//   - liveNav: renders "live" navigation links that support navigating without a page refresh
//   - liveViewTag: renders a container for a live.View - required for layoutTemplates
//   - liveFileInput: renders a file input tag for uploading files to a View
//   - liveImgPreview: renders an image preview for file to be uploaded to a View
//   - submitTag: renders a submit fuction that supports the PhxDisableWith feature
func Funcs() htmltmpl.FuncMap {
	return htmltmpl.FuncMap{
		"liveTitleTag":         TitleTag,
		"liveNav":              Navigation,
		"liveViewContainerTag": LiveViewTag,
		"liveFileInputTag":     FileInputTag,
		"liveImgPreviewTag":    ImagePreviewTag,
		"submitTag":            SubmitTag,
	}
}

// TitleTag renders a title tag that can be updated from Views.
func TitleTag(title, prefix, suffix string) (htmltmpl.HTML, error) {
	dot := map[string]any{
		"Title":  title,
		"Prefix": prefix,
		"Suffix": suffix,
	}
	tmpl := htmltmpl.Must(htmltmpl.New("liveTitleTag").Parse(
		`<title{{if .Prefix}} data-prefix="{{.Prefix}}"{{end}}{{if .Suffix}} data-suffix="{{.Suffix}}"{{end}}>{{.Prefix}}{{.Title}}{{.Suffix}}</title>`,
	))
	var buf strings.Builder
	err := tmpl.Execute(&buf, dot)
	if err != nil {
		return "", err
	}
	return htmltmpl.HTML(buf.String()), nil
}

// Navigation renders "live" navigation links that support navigating without a page refresh.
func Navigation(linkType, path string, params map[string]any, text string) (htmltmpl.HTML, error) {
	if linkType != "patch" && linkType != "navigate" {
		return "", fmt.Errorf("linkType must be 'patch' or 'navigate', got %q", linkType)
	}

	// TODO handle more than just text anchors
	tmpl := htmltmpl.Must(htmltmpl.New("liveNav").Parse(
		`<a data-phx-link="{{.LinkType}}" data-phx-link-state="push" href="{{.URL}}">{{.Text}}</a>`,
	))
	url, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	// add params to query string
	q := url.Query()
	for k, v := range params {
		// ignore empty key/value pairs
		if k != "" {
			q.Add(k, fmt.Sprint(v))
		}
	}
	url.RawQuery = q.Encode()
	if linkType == "navigate" {
		linkType = "redirect"
	}
	dot := map[string]any{
		"LinkType": linkType,
		"URL":      url,
		"Text":     text,
	}
	var buf strings.Builder
	err = tmpl.Execute(&buf, dot)
	if err != nil {
		return "", err
	}
	return htmltmpl.HTML(buf.String()), nil
}

// LiveView renders a container for a live.View - required for layoutTemplates.
func LiveViewTag(ld *LayoutDot) (htmltmpl.HTML, error) {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf(`
		<div
			data-phx-main="true"
			data-phx-session=""
			data-phx-static="%s"
			id="phx-%s">`, ld.Static, ld.LiveViewID))
	dot := &Dot{
		V:    ld.View,
		Meta: ld.Meta,
	}
	err := ld.ViewTemplate.ExecuteTemplate(&buf, ld.ViewTemplate.Name(), dot)
	if err != nil {
		return "", err
	}
	buf.WriteString("</div>")
	return htmltmpl.HTML(buf.String()), nil
}

// FileInputTag renders a file input tag for uploading files to a View.
func FileInputTag(uc UploadConfig) (htmltmpl.HTML, error) {
	tmpl := htmltmpl.Must(htmltmpl.New("liveFileInput").Parse(`<input
      id="{{.Ref}}"
      type="file"
      name="{{.Name}}"
      accept="{{.Accept}}"
      data-phx-active-refs="{{.ActiveRefs}}"
      data-phx-done-refs="{{.DoneRefs}}"
      data-phx-preflighted-refs="{{.PreflightedRefs}}"
      data-phx-update="ignore"
      data-phx-upload-ref="{{.Ref}}"
      phx-hook="Phoenix.LiveFileUpload"
      {{if .Multiple}}multiple{{end}} />`,
	))

	var activeRefs []string
	var doneRefs []string
	var preflightedRefs []string
	for _, e := range uc.Entries {
		activeRefs = append(activeRefs, e.Ref)
		if e.Done {
			doneRefs = append(doneRefs, e.Ref)
		}
		if e.Preflighted {
			preflightedRefs = append(preflightedRefs, e.Ref)
		}
	}
	dot := map[string]any{
		"Multiple":        uc.MaxEntries > 1,
		"Ref":             uc.Ref,
		"Name":            uc.Name,
		"Accept":          strings.Join(uc.Accept, ","),
		"ActiveRefs":      strings.Join(activeRefs, ","),
		"DoneRefs":        strings.Join(doneRefs, ","),
		"PreflightedRefs": strings.Join(preflightedRefs, ","),
	}
	var buf strings.Builder
	err := tmpl.Execute(&buf, dot)
	if err != nil {
		return "", err
	}
	return htmltmpl.HTML(buf.String()), nil
}

// ImagePreviewTag renders a preview image for a file upload.
func ImagePreviewTag(e UploadEntry) (htmltmpl.HTML, error) {
	tmpl := htmltmpl.Must(htmltmpl.New("liveImgPreview").Parse(`<img
      id="phx-preview-{{.Ref}}"
      data-phx-upload-ref="{{.UploadRef}}"
      data-phx-entry-ref="{{.Ref}}"
      data-phx-hook="Phoenix.LiveImgPreview"
      data-phx-update="ignore" />`,
	))
	dot := map[string]any{
		"Ref":       e.Ref,
		"UploadRef": e.UploadRef,
	}
	var buf strings.Builder
	err := tmpl.Execute(&buf, dot)
	if err != nil {
		return "", err
	}
	return htmltmpl.HTML(buf.String()), nil
}

// SubmitTag renders a submit button with support for the PhxDisableWith option.
func SubmitTag(label string, opts ...map[string]any) (htmltmpl.HTML, error) {
	tmpl := htmltmpl.Must(htmltmpl.New("submit").Parse(`<button
			type="submit"
			{{if .PhxDisableWith}}phx-disable-with="{{.PhxDisableWith}}"{{end}}
			{{if .Class}}class="{{.Class}}"{{end}}
			{{if .Disabled}}disabled{{end}}
		>{{.Label}}</button>`))
	dot := map[string]any{
		"Label": label,
	}
	if len(opts) > 0 {
		for k, v := range opts[0] {
			dot[k] = v
		}
	}
	var buf strings.Builder
	err := tmpl.Execute(&buf, dot)
	if err != nil {
		return "", err
	}
	return htmltmpl.HTML(buf.String()), nil
}
