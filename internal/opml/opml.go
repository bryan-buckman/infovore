// Package opml handles importing and exporting OPML files.
package opml

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
)

// OPML represents the root of an OPML document.
type OPML struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    Head     `xml:"head"`
	Body    Body     `xml:"body"`
}

// Head contains OPML metadata.
type Head struct {
	Title       string `xml:"title,omitempty"`
	DateCreated string `xml:"dateCreated,omitempty"`
}

// Body contains the outlines.
type Body struct {
	Outlines []Outline `xml:"outline"`
}

// Outline represents a single outline element (folder or feed).
type Outline struct {
	Text     string    `xml:"text,attr"`
	Title    string    `xml:"title,attr,omitempty"`
	Type     string    `xml:"type,attr,omitempty"`
	XMLURL   string    `xml:"xmlUrl,attr,omitempty"`
	HTMLURL  string    `xml:"htmlUrl,attr,omitempty"`
	Outlines []Outline `xml:"outline,omitempty"`
}

// FeedEntry represents a flattened feed with its folder path.
type FeedEntry struct {
	FolderPath []string // e.g., ["Tech", "Google"]
	Title      string
	URL        string
}

// Parse reads an OPML document and returns a flat list of FeedEntry.
func Parse(r io.Reader) ([]FeedEntry, error) {
	var doc OPML
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode opml: %w", err)
	}
	var entries []FeedEntry
	var walk func(outlines []Outline, path []string)
	walk = func(outlines []Outline, path []string) {
		for _, o := range outlines {
			if o.XMLURL != "" {
				// It's a feed.
				title := o.Title
				if title == "" {
					title = o.Text
				}
				entries = append(entries, FeedEntry{
					FolderPath: append([]string{}, path...),
					Title:      title,
					URL:        o.XMLURL,
				})
			} else if len(o.Outlines) > 0 {
				// It's a folder.
				name := o.Text
				if name == "" {
					name = o.Title
				}
				walk(o.Outlines, append(path, name))
			}
		}
	}
	walk(doc.Body.Outlines, nil)
	return entries, nil
}

// Export generates an OPML document from a nested map structure.
// folders should be a map of folder name -> sub-items.
func Export(title string, folders map[string][]FeedEntry) ([]byte, error) {
	doc := OPML{
		Version: "2.0",
		Head: Head{
			Title:       title,
			DateCreated: time.Now().Format(time.RFC1123Z),
		},
	}

	// Build outline tree.
	// For simplicity, we'll create a flat structure grouped by first folder level.
	folderOutlines := make(map[string]*Outline)
	var rootOutlines []Outline

	for _, entries := range folders {
		for _, e := range entries {
			feedOutline := Outline{
				Text:   e.Title,
				Title:  e.Title,
				Type:   "rss",
				XMLURL: e.URL,
			}
			if len(e.FolderPath) == 0 {
				rootOutlines = append(rootOutlines, feedOutline)
			} else {
				folderName := strings.Join(e.FolderPath, "/")
				if fo, ok := folderOutlines[folderName]; ok {
					fo.Outlines = append(fo.Outlines, feedOutline)
				} else {
					folderOutlines[folderName] = &Outline{
						Text:     e.FolderPath[0],
						Title:    e.FolderPath[0],
						Outlines: []Outline{feedOutline},
					}
				}
			}
		}
	}

	for _, fo := range folderOutlines {
		rootOutlines = append(rootOutlines, *fo)
	}
	doc.Body.Outlines = rootOutlines

	output, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), output...), nil
}
