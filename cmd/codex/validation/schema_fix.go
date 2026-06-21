// Schema-draft one-shot fixer.
//
// Performs mechanical, idempotent transforms on draft POI/encounter JSON
// files so legacy authoring shapes conform to the canonical types.
//
// DEPRECATED: This was authored to clean up a bulk of Gemini-generated
// drafts and is preserved here for the rare case more legacy drafts surface.
// Once the -draft directories are clean, this file can be deleted.
//
// Lifted from cmd/schemafix (now removed) so Codex is the single
// schema-tooling surface for game data.
package validation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

func fixLoadOrdered(path string) ([]byte, error) {
	return os.ReadFile(path)
}

type fixOrderedJSON struct {
	keys []string
	data map[string]json.RawMessage
}

func (o *fixOrderedJSON) UnmarshalJSON(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if t != json.Delim('{') {
		return fmt.Errorf("expected object, got %v", t)
	}
	o.data = map[string]json.RawMessage{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return err
		}
		k := t.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return err
		}
		o.keys = append(o.keys, k)
		o.data[k] = v
	}
	return nil
}

func (o *fixOrderedJSON) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
		buf.WriteString("  ")
		kb, _ := json.Marshal(k)
		buf.Write(kb)
		buf.WriteString(": ")
		buf.Write(o.data[k])
	}
	buf.WriteString("\n}\n")
	return buf.Bytes(), nil
}

func (o *fixOrderedJSON) renameKey(from, to string) bool {
	if _, ok := o.data[from]; !ok {
		return false
	}
	if _, ok := o.data[to]; ok {
		delete(o.data, from)
		o.keys = fixRemoveKey(o.keys, from)
		return true
	}
	o.data[to] = o.data[from]
	delete(o.data, from)
	for i, k := range o.keys {
		if k == from {
			o.keys[i] = to
		}
	}
	return true
}

func (o *fixOrderedJSON) dropKey(k string) bool {
	if _, ok := o.data[k]; !ok {
		return false
	}
	delete(o.data, k)
	o.keys = fixRemoveKey(o.keys, k)
	return true
}

func (o *fixOrderedJSON) setKey(k string, v json.RawMessage) {
	if _, ok := o.data[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.data[k] = v
}

func fixRemoveKey(s []string, k string) []string {
	out := s[:0]
	for _, x := range s {
		if x != k {
			out = append(out, x)
		}
	}
	return out
}

func fixPOI(o *fixOrderedJSON) (changed bool) {
	if o.renameKey("type", "category") {
		changed = true
	}
	if fixNormalizeAny(&o.data) {
		changed = true
	}
	if discoveryRaw, ok := o.data["discovery"]; ok {
		if fixed, ok := fixNormalizeDiscovery(discoveryRaw); ok {
			o.data["discovery"] = fixed
			changed = true
		}
	}
	return
}

func fixNormalizeAny(m *map[string]json.RawMessage) bool {
	changed := false
	for k, raw := range *m {
		newRaw, c := fixNormalizeRaw(raw)
		if c {
			(*m)[k] = newRaw
			changed = true
		}
	}
	return changed
}

func fixNormalizeRaw(raw json.RawMessage) (json.RawMessage, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return raw, false
	}
	switch trimmed[0] {
	case '{':
		return fixNormalizeObject(raw)
	case '[':
		return fixNormalizeArray(raw)
	}
	return raw, false
}

func fixNormalizeArray(raw json.RawMessage) (json.RawMessage, bool) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return raw, false
	}
	changed := false
	for i, item := range arr {
		if fixed, ok := fixRepairMangledLootEntry(item); ok {
			arr[i] = fixed
			changed = true
			continue
		}
		newItem, c := fixNormalizeRaw(item)
		if c {
			arr[i] = newItem
			changed = true
		}
	}
	if !changed {
		return raw, false
	}
	out, _ := json.Marshal(arr)
	return out, true
}

func fixRepairMangledLootEntry(raw json.RawMessage) (json.RawMessage, bool) {
	var o fixOrderedJSON
	if err := json.Unmarshal(raw, &o); err != nil {
		return raw, false
	}
	if len(o.keys) == 1 && o.keys[0] == "gold" {
		var n int
		if json.Unmarshal(o.data["gold"], &n) == nil {
			out := fmt.Sprintf(`{"item":"gold-piece","quantity":%d}`, n)
			return json.RawMessage(out), true
		}
	}
	if len(o.keys) == 1 && o.keys[0] == "items" {
		var items []struct {
			ID       string `json:"id"`
			Quantity any    `json:"quantity"`
		}
		if json.Unmarshal(o.data["items"], &items) == nil && len(items) == 1 {
			qb, _ := json.Marshal(items[0].Quantity)
			out := fmt.Sprintf(`{"item":%q,"quantity":%s}`, items[0].ID, string(qb))
			return json.RawMessage(out), true
		}
	}
	return raw, false
}

func fixNormalizeObject(raw json.RawMessage) (json.RawMessage, bool) {
	var o fixOrderedJSON
	if err := json.Unmarshal(raw, &o); err != nil {
		return raw, false
	}
	changed := false

	for k, v := range o.data {
		newV, c := fixNormalizeRaw(v)
		if c {
			o.data[k] = newV
			changed = true
		}
	}

	if typeRaw, hasType := o.data["type"]; hasType {
		var typeStr string
		if json.Unmarshal(typeRaw, &typeStr) == nil {
			if fixNormalizeRequirement(&o, typeStr) {
				changed = true
			}
		}
	}

	if effRaw, ok := o.data["effect"]; ok {
		var s string
		if json.Unmarshal(effRaw, &s) == nil {
			obj := map[string]string{"id": s}
			b, _ := json.Marshal(obj)
			o.data["effect"] = b
			changed = true
		}
	}

	if _, ok := o.data["role"]; ok {
		if _, hasTitle := o.data["title"]; !hasTitle {
			o.renameKey("role", "title")
		} else {
			o.dropKey("role")
		}
		changed = true
	}

	if costRaw, ok := o.data["cost"]; ok {
		var c fixOrderedJSON
		if json.Unmarshal(costRaw, &c) == nil {
			if itemRaw, hasItem := c.data["item"]; hasItem {
				if qRaw, hasQ := c.data["quantity"]; hasQ {
					var item string
					var qty int
					if json.Unmarshal(itemRaw, &item) == nil && json.Unmarshal(qRaw, &qty) == nil {
						if item == "gold-piece" || item == "gold" {
							b, _ := json.Marshal(qty)
							c.data = map[string]json.RawMessage{"gold": b}
							c.keys = []string{"gold"}
						} else {
							items := []map[string]any{{"id": item, "quantity": qty}}
							b, _ := json.Marshal(items)
							c.data = map[string]json.RawMessage{"items": b}
							c.keys = []string{"items"}
						}
						o.data["cost"] = fixEncodeOrdered(&c)
						changed = true
					}
				}
			}
		}
	}

	if effRaw, ok := o.data["effect"]; ok {
		var inner fixOrderedJSON
		if json.Unmarshal(effRaw, &inner) == nil {
			if _, hasID := inner.data["id"]; !hasID {
				if _, hasType := inner.data["type"]; hasType {
					inner.renameKey("type", "id")
					o.data["effect"] = fixEncodeOrdered(&inner)
					changed = true
				}
			}
		}
	}

	if typeRaw, ok := o.data["type"]; ok {
		var t string
		if json.Unmarshal(typeRaw, &t) == nil {
			if t == "next" || t == "event" {
				b, _ := json.Marshal("narrative")
				o.data["type"] = b
				changed = true
			}
		}
	}
	if _, ok := o.data["event_id"]; ok {
		o.dropKey("event_id")
		changed = true
	}

	if _, hasLabel := o.data["label"]; hasLabel {
		if _, hasNext := o.data["next"]; hasNext {
			if _, hasType := o.data["type"]; !hasType {
				if o.dropKey("is_terminal") {
					changed = true
				}
				if reqRaw, ok := o.data["requirement"]; ok {
					arr := []json.RawMessage{reqRaw}
					b, _ := json.Marshal(arr)
					o.data["requirements"] = b
					o.keys = append(o.keys, "requirements")
					o.dropKey("requirement")
					changed = true
				}
			}
		}
	}

	if ltRaw, ok := o.data["loot_table"]; ok {
		var lt fixOrderedJSON
		if json.Unmarshal(ltRaw, &lt) == nil {
			ltChanged := false
			if xpRaw, has := lt.data["xp"]; has {
				var xp int
				if json.Unmarshal(xpRaw, &xp) == nil {
					fixMergeRewardField(&o, "xp", xp)
					lt.dropKey("xp")
					ltChanged = true
					changed = true
				}
			}
			if costRaw, has := lt.data["cost"]; has {
				if _, exists := o.data["cost"]; !exists {
					o.keys = append(o.keys, "cost")
				}
				o.data["cost"] = costRaw
				lt.dropKey("cost")
				ltChanged = true
				changed = true
			}
			if ltChanged {
				o.data["loot_table"] = fixEncodeOrdered(&lt)
			}
		}
	}

	if ltRaw, ok := o.data["loot_table"]; ok {
		var s string
		if json.Unmarshal(ltRaw, &s) == nil {
			placeholder := fmt.Sprintf(`{"guaranteed":[{"item":"%s","quantity":1}]}`, s)
			o.data["loot_table"] = json.RawMessage(placeholder)
			changed = true
		}
	}

	if !changed {
		return raw, false
	}
	out := fixEncodeOrdered(&o)
	return out, true
}

func fixMergeRewardField(o *fixOrderedJSON, key string, val int) {
	var r fixOrderedJSON
	if existing, ok := o.data["reward"]; ok {
		_ = json.Unmarshal(existing, &r)
	} else {
		r.data = map[string]json.RawMessage{}
	}
	b, _ := json.Marshal(val)
	if _, ok := r.data[key]; !ok {
		r.keys = append(r.keys, key)
	}
	r.data[key] = b
	if _, exists := o.data["reward"]; !exists {
		o.keys = append(o.keys, "reward")
	}
	o.data["reward"] = fixEncodeOrdered(&r)
}

func fixEncodeOrdered(o *fixOrderedJSON) []byte {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range o.keys {
		if _, ok := o.data[k]; !ok {
			continue
		}
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(o.data[k])
	}
	buf.WriteByte('}')
	return buf.Bytes()
}

func fixNormalizeRequirement(o *fixOrderedJSON, typeStr string) bool {
	changed := false

	if _, ok := o.data["key"]; ok {
		o.renameKey("key", "id")
		changed = true
	}

	if typeStr == "item" {
		if qRaw, ok := o.data["quantity"]; ok {
			var n int
			if json.Unmarshal(qRaw, &n) == nil {
				b, _ := json.Marshal(n)
				delete(o.data, "quantity")
				o.keys = fixRemoveKey(o.keys, "quantity")
				if _, ok := o.data["min"]; !ok {
					o.keys = append(o.keys, "min")
				}
				o.data["min"] = b
				changed = true
			}
		}
	}

	valRaw, hasVal := o.data["value"]
	if !hasVal {
		return changed
	}

	switch typeStr {
	case "skill", "stat", "level", "quest_points", "experience", "qp":
		var n int
		if json.Unmarshal(valRaw, &n) == nil {
			b, _ := json.Marshal(n)
			delete(o.data, "value")
			o.keys = fixRemoveKey(o.keys, "value")
			if _, ok := o.data["min"]; !ok {
				o.keys = append(o.keys, "min")
			}
			o.data["min"] = b
			changed = true
		}
	case "class", "race", "alignment", "background", "deity":
		var s string
		if json.Unmarshal(valRaw, &s) == nil {
			b, _ := json.Marshal([]string{s})
			delete(o.data, "value")
			o.keys = fixRemoveKey(o.keys, "value")
			if _, ok := o.data["values"]; !ok {
				o.keys = append(o.keys, "values")
			}
			o.data["values"] = b
			changed = true
		}
	case "item":
		delete(o.data, "value")
		o.keys = fixRemoveKey(o.keys, "value")
		changed = true
	}

	if vs, ok := o.data["values"]; ok {
		var s string
		if json.Unmarshal(vs, &s) == nil {
			b, _ := json.Marshal([]string{s})
			o.data["values"] = b
			changed = true
		}
	}
	return changed
}

func fixNormalizeDiscovery(raw json.RawMessage) (json.RawMessage, bool) {
	var o fixOrderedJSON
	if err := json.Unmarshal(raw, &o); err != nil {
		return raw, false
	}
	changed := false
	if o.renameKey("base_chance", "chance") {
		changed = true
	}
	if pc, ok := o.data["passive_check"]; ok {
		var inner fixOrderedJSON
		if json.Unmarshal(pc, &inner) == nil {
			for _, k := range []string{"skill", "dc"} {
				if v, ok := inner.data[k]; ok {
					if _, exists := o.data[k]; !exists {
						o.keys = append(o.keys, k)
					}
					o.data[k] = v
				}
			}
			delete(o.data, "passive_check")
			o.keys = fixRemoveKey(o.keys, "passive_check")
			changed = true
		}
	}
	if !changed {
		return raw, false
	}
	return fixEncodeOrdered(&o), true
}

func fixEncounter(o *fixOrderedJSON) (changed bool) {
	if o.dropKey("type") {
		changed = true
	}
	if o.dropKey("index") {
		changed = true
	}
	if raw, ok := o.data["location_type"]; ok {
		var lt string
		if err := json.Unmarshal(raw, &lt); err == nil {
			var trigger string
			switch lt {
			case "environment":
				trigger = "travel"
			case "city", "settlement":
				trigger = "location"
			case "building":
				trigger = "building"
			case "building_type":
				trigger = "building_type"
			default:
				trigger = "travel"
			}
			tb, _ := json.Marshal(trigger)
			o.setKey("trigger", tb)
			o.dropKey("location_type")
			changed = true
		}
	}
	if fixNormalizeAny(&o.data) {
		changed = true
	}
	return
}

func fixProcess(path string, fix func(*fixOrderedJSON) bool, res *FixSchemaResult) {
	b, err := fixLoadOrdered(path)
	if err != nil {
		res.Failures = append(res.Failures, fmt.Sprintf("read %s: %v", path, err))
		return
	}
	var o fixOrderedJSON
	if err := json.Unmarshal(b, &o); err != nil {
		res.Failures = append(res.Failures, fmt.Sprintf("parse %s: %v", path, err))
		return
	}
	if !fix(&o) {
		return
	}
	out, err := o.Marshal()
	if err != nil {
		res.Failures = append(res.Failures, fmt.Sprintf("marshal %s: %v", path, err))
		return
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		res.Failures = append(res.Failures, fmt.Sprintf("write %s: %v", path, err))
		return
	}
	res.Fixed = append(res.Fixed, path)
}

// FixSchemaResult summarises a one-shot legacy fix run.
type FixSchemaResult struct {
	Fixed    []string `json:"fixed"`
	Failures []string `json:"failures"`
}

// FixSchemaDirs walks the canonical draft directories and applies legacy
// transforms. Deprecated — see file-level comment.
func FixSchemaDirs() (*FixSchemaResult, error) {
	res := &FixSchemaResult{}
	d := DefaultSchemaDirs()
	for _, p := range schemaWalkJSON(d.POIs) {
		fixProcess(p, fixPOI, res)
	}
	for _, p := range schemaWalkJSON(d.Encounters) {
		fixProcess(p, fixEncounter, res)
	}
	return res, nil
}
