package subsonic

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
)

// Obj is an insertion-ordered map that serialises to both Subsonic JSON and
// Subsonic XML (scalars become attributes, nested objects become elements).
type Obj struct {
	keys []string
	vals map[string]any
}

func NewObj() *Obj { return &Obj{vals: map[string]any{}} }

func (o *Obj) Set(k string, v any) *Obj {
	if o.vals == nil {
		o.vals = map[string]any{}
	}
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
	return o
}

func (o *Obj) SetIf(k string, v any, cond bool) *Obj {
	if cond {
		o.Set(k, v)
	}
	return o
}

func (o *Obj) Get(k string) any { return o.vals[k] }

// Append adds an item to an array-valued field, creating it on first use.
func (o *Obj) Append(k string, v any) *Obj {
	if o.vals == nil {
		o.vals = map[string]any{}
	}
	cur, ok := o.vals[k]
	if !ok {
		o.keys = append(o.keys, k)
		o.vals[k] = []*Obj{}
		cur = o.vals[k]
	}
	switch arr := cur.(type) {
	case []*Obj:
		if item, ok := v.(*Obj); ok {
			o.vals[k] = append(arr, item)
		}
	case []any:
		o.vals[k] = append(arr, v)
	}
	return o
}

func (o *Obj) Len() int { return len(o.keys) }

// namedList is an array whose JSON form is a plain array but whose XML form
// nests each item under a distinct element name. OpenSubsonic uses this for
// "openSubsonicExtensions": an array in JSON, <extension> children in XML.
type namedList struct {
	items []*Obj
	name  string
}

func (n namedList) MarshalJSON() ([]byte, error) {
	if n.items == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(n.items)
}

func (o *Obj) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	first := true
	for _, k := range o.keys {
		v := o.vals[k]
		if v == nil {
			continue
		}
		if !first {
			buf.WriteByte(',')
		}
		first = false
		kb, _ := json.Marshal(k)
		buf.Write(kb)
		buf.WriteByte(':')
		vb, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func isScalar(v any) bool {
	switch v.(type) {
	case nil, string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	}
	return false
}

func scalarString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	default:
		return fmt.Sprint(t)
	}
}

// MarshalXML renders the object as an XML element whose scalar fields are
// attributes and whose object/array fields are child elements.
func (o *Obj) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	for _, k := range o.keys {
		v := o.vals[k]
		if v == nil {
			continue
		}
		if isScalar(v) {
			start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: k}, Value: scalarString(v)})
		}
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, k := range o.keys {
		v := o.vals[k]
		if v == nil || isScalar(v) {
			continue
		}
		switch t := v.(type) {
		case *Obj:
			if err := e.EncodeElement(t, xml.StartElement{Name: xml.Name{Local: k}}); err != nil {
				return err
			}
		case []*Obj:
			for _, item := range t {
				if err := e.EncodeElement(item, xml.StartElement{Name: xml.Name{Local: k}}); err != nil {
					return err
				}
			}
		case []any:
			for _, item := range t {
				if sub, ok := item.(*Obj); ok {
					if err := e.EncodeElement(sub, xml.StartElement{Name: xml.Name{Local: k}}); err != nil {
						return err
					}
				}
			}
		case namedList:
			wrap := xml.StartElement{Name: xml.Name{Local: k}}
			if err := e.EncodeToken(wrap); err != nil {
				return err
			}
			for _, item := range t.items {
				if err := e.EncodeElement(item, xml.StartElement{Name: xml.Name{Local: t.name}}); err != nil {
					return err
				}
			}
			if err := e.EncodeToken(wrap.End()); err != nil {
				return err
			}
		case []int:
			for _, n := range t {
				if err := e.EncodeElement(n, xml.StartElement{Name: xml.Name{Local: k}}); err != nil {
					return err
				}
			}
		case []string:
			for _, str := range t {
				if err := e.EncodeElement(str, xml.StartElement{Name: xml.Name{Local: k}}); err != nil {
					return err
				}
			}
		}
	}
	return e.EncodeToken(start.End())
}

// sortedIndexes returns deterministic index letters for getArtists.
func sortedIndexes(m map[string]*Obj) []*Obj {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i] == "#" {
			return false
		}
		if keys[j] == "#" {
			return true
		}
		return keys[i] < keys[j]
	})
	out := make([]*Obj, 0, len(keys))
	for _, k := range keys {
		out = append(out, m[k])
	}
	return out
}
