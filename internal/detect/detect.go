package detect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gopkg.in/yaml.v3"
)

// printer localizes schema error messages (English).
var printer = message.NewPrinter(language.English)

// Issue is a single validation problem within a file.
type Issue struct {
	// Doc is the 1-based YAML document index within the file (Sigma files may
	// contain multiple rules separated by "---").
	Doc int
	// Msg is a human-readable description, e.g. "/detection: missing property".
	Msg string
}

func (i Issue) String() string {
	if i.Doc > 1 {
		return fmt.Sprintf("doc %d: %s", i.Doc, i.Msg)
	}
	return i.Msg
}

// FileResult is the validation outcome for one rule file.
type FileResult struct {
	Path   string
	Valid  bool
	Issues []Issue
}

// ValidateBytes validates one or more Sigma rule documents from raw YAML.
func ValidateBytes(data []byte) ([]Issue, error) {
	sch, err := schema()
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))

	var issues []Issue
	docIndex := 0
	for {
		var raw any
		decErr := dec.Decode(&raw)
		if decErr == io.EOF {
			break
		}
		docIndex++
		if decErr != nil {
			issues = append(issues, Issue{Doc: docIndex, Msg: "YAML parse error: " + decErr.Error()})
			break
		}
		if raw == nil {
			continue // empty document
		}
		inst, convErr := toJSONValue(raw)
		if convErr != nil {
			issues = append(issues, Issue{Doc: docIndex, Msg: convErr.Error()})
			continue
		}
		if vErr := sch.Validate(inst); vErr != nil {
			var ve *jsonschema.ValidationError
			if asValidationError(vErr, &ve) {
				for _, line := range flatten(ve) {
					issues = append(issues, Issue{Doc: docIndex, Msg: line})
				}
			} else {
				issues = append(issues, Issue{Doc: docIndex, Msg: vErr.Error()})
			}
		}
	}
	if docIndex == 0 {
		issues = append(issues, Issue{Doc: 0, Msg: "no YAML documents found"})
	}
	return issues, nil
}

// ValidateFile validates a single rule file.
func ValidateFile(path string) FileResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return FileResult{Path: path, Valid: false, Issues: []Issue{{Msg: "cannot read file: " + err.Error()}}}
	}
	issues, err := ValidateBytes(data)
	if err != nil {
		return FileResult{Path: path, Valid: false, Issues: []Issue{{Msg: err.Error()}}}
	}
	return FileResult{Path: path, Valid: len(issues) == 0, Issues: issues}
}

// ValidateDir validates every .yml/.yaml file under dir. When recursive is
// false only the top level is scanned.
func ValidateDir(dir string, recursive bool) []FileResult {
	pattern := "*.{yml,yaml}"
	if recursive {
		pattern = "**/*.{yml,yaml}"
	}
	fsys := os.DirFS(dir)
	var paths []string
	_ = doublestar.GlobWalk(fsys, pattern, func(p string, _ os.DirEntry) error {
		paths = append(paths, p)
		return nil
	})
	sort.Strings(paths)

	results := make([]FileResult, 0, len(paths))
	for _, p := range paths {
		full := p
		if dir != "" {
			full = dir + string(os.PathSeparator) + p
		}
		results = append(results, ValidateFile(full))
	}
	return results
}

// toJSONValue normalizes a YAML-decoded value into the JSON-compatible types
// the validator expects (round-tripping through JSON for correct numeric and
// key types).
func toJSONValue(v any) (any, error) {
	b, err := json.Marshal(normalize(v))
	if err != nil {
		return nil, fmt.Errorf("cannot convert YAML to JSON: %w", err)
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

// normalize coerces map[any]any (possible from some YAML inputs) into
// map[string]any recursively so the value is JSON-marshalable.
func normalize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[k] = normalize(val)
		}
		return m
	case map[any]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[fmt.Sprint(k)] = normalize(val)
		}
		return m
	case []any:
		s := make([]any, len(t))
		for i, val := range t {
			s[i] = normalize(val)
		}
		return s
	case time.Time:
		// YAML coerces unquoted dates ("2026-06-01") and datetimes into
		// time.Time. Render them back to strings so Sigma's string/date fields
		// (date, modified) validate as the author intended, rather than failing
		// because JSON marshaled a full RFC3339 timestamp.
		if t.Equal(t.Truncate(24 * time.Hour)) {
			return t.Format("2006-01-02")
		}
		return t.Format(time.RFC3339)
	default:
		return v
	}
}

// flatten walks a ValidationError to its leaves, producing one message per
// concrete failure with its instance location.
func flatten(e *jsonschema.ValidationError) []string {
	var out []string
	var walk func(ve *jsonschema.ValidationError)
	walk = func(ve *jsonschema.ValidationError) {
		if len(ve.Causes) == 0 {
			loc := "/" + strings.Join(ve.InstanceLocation, "/")
			if loc == "/" {
				loc = "(root)"
			}
			out = append(out, loc+": "+ve.ErrorKind.LocalizedString(printer))
			return
		}
		for _, c := range ve.Causes {
			walk(c)
		}
	}
	walk(e)
	return out
}

// asValidationError reports whether err is a *jsonschema.ValidationError and,
// if so, stores it in target.
func asValidationError(err error, target **jsonschema.ValidationError) bool {
	ve, ok := err.(*jsonschema.ValidationError)
	if ok {
		*target = ve
	}
	return ok
}
