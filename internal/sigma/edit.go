// Package sigma provides comment- and order-preserving structural edits to a
// single Sigma rule document. The field-editing tools build on it so the agent
// can change one field at a time instead of rewriting the whole YAML file —
// which keeps explanatory inline comments (e.g. why a filter exists) intact and
// spends far fewer tokens.
//
// Edits operate on a yaml.Node tree (not a decoded map) precisely so comments
// and key order survive a round trip.
package sigma

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Editor wraps a single decoded YAML document for structural editing.
type Editor struct {
	doc yaml.Node
}

// Load parses exactly one YAML document. Multi-document files are rejected so
// edits never silently touch the wrong rule.
func Load(data []byte) (*Editor, error) {
	e := &Editor{}
	if err := yaml.Unmarshal(data, &e.doc); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	if e.doc.Kind == 0 || len(e.doc.Content) == 0 {
		// Empty document: start a fresh mapping.
		e.doc.Kind = yaml.DocumentNode
		e.doc.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
		return e, nil
	}
	if e.root().Kind != yaml.MappingNode {
		return nil, fmt.Errorf("rule is not a YAML mapping")
	}
	return e, nil
}

// Bytes serializes the document back to YAML.
func (e *Editor) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&e.doc); err != nil {
		return nil, err
	}
	_ = enc.Close()
	return buf.Bytes(), nil
}

// root returns the top-level mapping node.
func (e *Editor) root() *yaml.Node { return e.doc.Content[0] }

// SetScalar sets a top-level key to a scalar value, creating it if absent.
func (e *Editor) SetScalar(key, value string) {
	mapSet(e.root(), key, scalar(value))
}

// SetNode encodes an arbitrary Go value and sets it at a top-level key.
func (e *Editor) SetNode(key string, v any) error {
	n, err := encode(v)
	if err != nil {
		return err
	}
	mapSet(e.root(), key, n)
	return nil
}

// Delete removes a top-level key. Reports whether it existed.
func (e *Editor) Delete(key string) bool { return mapDelete(e.root(), key) }

// Has reports whether a top-level key exists.
func (e *Editor) Has(key string) bool {
	_, idx := mapGet(e.root(), key)
	return idx >= 0
}

// EnsureMapping returns the mapping node at a top-level key, creating an empty
// mapping if the key is missing.
func (e *Editor) EnsureMapping(key string) (*yaml.Node, error) {
	v, idx := mapGet(e.root(), key)
	if idx < 0 {
		v = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		mapSet(e.root(), key, v)
		return v, nil
	}
	if v.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%q is not a mapping", key)
	}
	return v, nil
}

// SetInMapping sets key=value (encoded) inside a child mapping node.
func SetInMapping(m *yaml.Node, key string, v any) error {
	n, err := encode(v)
	if err != nil {
		return err
	}
	mapSet(m, key, n)
	return nil
}

// SetScalarInMapping sets key to a scalar string inside a child mapping node.
func SetScalarInMapping(m *yaml.Node, key, value string) {
	mapSet(m, key, scalar(value))
}

// DeleteInMapping removes a key from a child mapping. Reports whether it existed.
func DeleteInMapping(m *yaml.Node, key string) bool { return mapDelete(m, key) }

// AppendListItem appends an encoded value to the sequence at a top-level key,
// creating the sequence if absent.
func (e *Editor) AppendListItem(key string, v any) error {
	seq, err := e.ensureSeq(key)
	if err != nil {
		return err
	}
	n, err := encode(v)
	if err != nil {
		return err
	}
	seq.Content = append(seq.Content, n)
	return nil
}

// RemoveListItem removes the first sequence item at a top-level key whose
// scalar value equals value. Reports whether something was removed.
func (e *Editor) RemoveListItem(key, value string) (bool, error) {
	v, idx := mapGet(e.root(), key)
	if idx < 0 {
		return false, nil
	}
	if v.Kind != yaml.SequenceNode {
		return false, fmt.Errorf("%q is not a list", key)
	}
	for i, item := range v.Content {
		if item.Kind == yaml.ScalarNode && item.Value == value {
			v.Content = append(v.Content[:i], v.Content[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

// RemoveMapItem removes the first sequence item at a top-level key that is a
// mapping whose scalar child field equals value (e.g. remove the `tests:` entry
// whose name is "root login fires"). Reports whether something was removed.
func (e *Editor) RemoveMapItem(key, field, value string) (bool, error) {
	v, idx := mapGet(e.root(), key)
	if idx < 0 {
		return false, nil
	}
	if v.Kind != yaml.SequenceNode {
		return false, fmt.Errorf("%q is not a list", key)
	}
	for i, item := range v.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		if child, ci := mapGet(item, field); ci >= 0 && child.Kind == yaml.ScalarNode && child.Value == value {
			v.Content = append(v.Content[:i], v.Content[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

// List returns the sequence node at a top-level key, if present.
func (e *Editor) List(key string) (*yaml.Node, bool) {
	v, idx := mapGet(e.root(), key)
	if idx < 0 || v.Kind != yaml.SequenceNode {
		return nil, false
	}
	return v, true
}

func (e *Editor) ensureSeq(key string) (*yaml.Node, error) {
	v, idx := mapGet(e.root(), key)
	if idx < 0 {
		v = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		mapSet(e.root(), key, v)
		return v, nil
	}
	if v.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%q is not a list", key)
	}
	return v, nil
}

// --- low-level node helpers ---

// mapGet finds a key in a mapping node, returning its value node and the index
// of the key node (or -1 if absent).
func mapGet(m *yaml.Node, key string) (*yaml.Node, int) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1], i
		}
	}
	return nil, -1
}

// mapSet replaces the value for key, or appends a new key/value pair.
func mapSet(m *yaml.Node, key string, val *yaml.Node) {
	if _, idx := mapGet(m, key); idx >= 0 {
		m.Content[idx+1] = val
		return
	}
	m.Content = append(m.Content, scalar(key), val)
}

// mapDelete removes a key (and its value) from a mapping node.
func mapDelete(m *yaml.Node, key string) bool {
	if _, idx := mapGet(m, key); idx >= 0 {
		m.Content = append(m.Content[:idx], m.Content[idx+2:]...)
		return true
	}
	return false
}

// scalar builds a scalar node, using a literal block style for multi-line
// strings so descriptions and runbooks render readably.
func scalar(value string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
	if bytes.ContainsRune([]byte(value), '\n') {
		n.Style = yaml.LiteralStyle
	}
	return n
}

// encode turns an arbitrary Go value into a yaml.Node tree.
func encode(v any) (*yaml.Node, error) {
	if n, ok := v.(*yaml.Node); ok {
		return n, nil
	}
	var n yaml.Node
	if err := n.Encode(v); err != nil {
		return nil, err
	}
	return &n, nil
}
