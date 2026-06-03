// Package config: reflection metadata for fast environment application.
//
// This module precomputes a compact descriptor for a config struct type that
// maps environment variable suffixes (segments joined by "_") to struct field
// index paths and kinds. The apply path then reuses this metadata to minimize
// reflection overhead and string concatenations.
package config

import (
	"os"
	"reflect"
	"strings"
	"sync"
	"time"
)

// envMeta holds flattened leaves for direct/ptr scalar fields and a set of
// pointer-to-struct nodes that need conditional allocation based on env vars.
// The env var names are stored as suffixes (without prefix) to be combined
// with the runtime prefix cheaply.

type envLeafKind uint8

const (
	leafString envLeafKind = iota
	leafBool
	leafInt
	leafUint
	leafDuration
	leafPtrString
	leafPtrBool
	leafPtrInt
	leafPtrUint
	leafPtrDuration
)

type envLeaf struct {
	index      []int  // reflect index path to the field
	fullSuffix string // e.g., "INNER_STR" or "S"
	kind       envLeafKind
}

type ptrStructNode struct {
	index      []int  // index path of the pointer-to-struct field
	baseSuffix string // e.g., "PINNER" for PtrInner `env:"PINNER"`
	leafIdxs   []int  // indices into envMeta.leaves belonging to this subtree
}

type envMeta struct {
	structType reflect.Type // underlying struct type
	leaves     []envLeaf
	ptrNodes   []ptrStructNode
}

var envMetaCache sync.Map // map[reflect.Type]*envMeta

func getEnvMeta(t reflect.Type) *envMeta {
	if t == nil {
		return nil
	}
	// Root must be a struct type
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	if v, ok := envMetaCache.Load(t); ok {
		return v.(*envMeta)
	}
	m := buildEnvMeta(t)
	if m == nil {
		return nil
	}
	actual, _ := envMetaCache.LoadOrStore(t, m)
	return actual.(*envMeta)
}

func buildEnvMeta(t reflect.Type) *envMeta {
	m := &envMeta{structType: t}
	var leaves []envLeaf
	var ptrs []ptrStructNode

	var walk func(rt reflect.Type, idxBase []int, segs []string)
	walk = func(rt reflect.Type, idxBase []int, segs []string) {
		if rt.Kind() != reflect.Struct {
			return
		}
		for i := 0; i < rt.NumField(); i++ {
			sf := rt.Field(i)
			if sf.PkgPath != "" { // unexported
				continue
			}
			tag := sf.Tag.Get(envVarTagName)
			if tag == "-" {
				continue
			}
			seg := tag
			if seg == "" {
				seg = toScreamingSnake(sf.Name)
			}
			idx := append(append([]int(nil), idxBase...), sf.Index...)
			ft := sf.Type
			switch ft.Kind() {
			case reflect.Struct:
				walk(ft, idx, append(segs, seg))
			case reflect.String:
				leaves = append(leaves, envLeaf{index: idx, fullSuffix: joinSegs(append(segs, seg)), kind: leafString})
			case reflect.Bool:
				leaves = append(leaves, envLeaf{index: idx, fullSuffix: joinSegs(append(segs, seg)), kind: leafBool})
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if ft == reflect.TypeOf(time.Duration(0)) {
					leaves = append(leaves, envLeaf{index: idx, fullSuffix: joinSegs(append(segs, seg)), kind: leafDuration})
				} else {
					leaves = append(leaves, envLeaf{index: idx, fullSuffix: joinSegs(append(segs, seg)), kind: leafInt})
				}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				leaves = append(leaves, envLeaf{index: idx, fullSuffix: joinSegs(append(segs, seg)), kind: leafUint})
			case reflect.Pointer:
				el := ft.Elem()
				switch el.Kind() {
				case reflect.Struct:
					// Track ptr struct node; record leaves added by subtree.
					start := len(leaves)
					walk(el, idx, append(segs, seg))
					ptrs = append(ptrs, ptrStructNode{
						index:      idx,
						baseSuffix: joinSegs(append(segs, seg)),
						leafIdxs:   rangeIdx(start, len(leaves)-1),
					})
				case reflect.String:
					leaves = append(leaves, envLeaf{index: idx, fullSuffix: joinSegs(append(segs, seg)), kind: leafPtrString})
				case reflect.Bool:
					leaves = append(leaves, envLeaf{index: idx, fullSuffix: joinSegs(append(segs, seg)), kind: leafPtrBool})
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					if el == reflect.TypeOf(time.Duration(0)) {
						leaves = append(leaves, envLeaf{index: idx, fullSuffix: joinSegs(append(segs, seg)), kind: leafPtrDuration})
					} else {
						leaves = append(leaves, envLeaf{index: idx, fullSuffix: joinSegs(append(segs, seg)), kind: leafPtrInt})
					}
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					leaves = append(leaves, envLeaf{index: idx, fullSuffix: joinSegs(append(segs, seg)), kind: leafPtrUint})
				}
			}
		}
	}

	walk(t, nil, nil)
	m.leaves = leaves
	m.ptrNodes = ptrs
	return m
}

func rangeIdx(start, end int) []int {
	if end < start {
		return nil
	}
	idxs := make([]int, end-start+1)
	for i := range idxs {
		idxs[i] = start + i
	}
	return idxs
}

func joinSegs(segs []string) string {
	if len(segs) == 0 {
		return ""
	}
	if len(segs) == 1 {
		return segs[0]
	}
	var b strings.Builder
	// pre-size approximate: segments + underscores
	llen := 0
	for _, s := range segs {
		llen += len(s)
	}
	b.Grow(llen + len(segs) - 1)
	for i, s := range segs {
		if i > 0 {
			b.WriteByte('_')
		}
		b.WriteString(s)
	}
	return b.String()
}

// applyEnvWithMeta applies env overrides to a struct value using precomputed metadata.
func applyEnvWithMeta(v reflect.Value, prefix string, strategy SetStrategy) {
	if !v.IsValid() {
		return
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	m := getEnvMeta(v.Type())
	if m == nil {
		return
	}
	// Precompute prefix formatting closure.
	var nameOf func(suffix string) string
	if prefix == "" {
		nameOf = func(suffix string) string { return suffix }
	} else {
		pfx := prefix + "_"
		nameOf = func(suffix string) string { return pfx + suffix }
	}
	// Allocate pointer-to-struct nodes only if any descendant env var is present.
	for i := range m.ptrNodes {
		n := &m.ptrNodes[i]
		fv := v.FieldByIndex(n.index)
		if !fv.IsValid() || fv.Kind() != reflect.Pointer || !fv.CanSet() || !fv.IsNil() {
			continue
		}
		present := false
		// Check descendant leaves
		for _, li := range n.leafIdxs {
			leaf := m.leaves[li]
			if leaf.fullSuffix == "" { // safety
				continue
			}
			if _, ok := os.LookupEnv(nameOf(leaf.fullSuffix)); ok {
				present = true
				break
			}
		}
		if present {
			fv.Set(reflect.New(fv.Type().Elem()))
		}
	}
	// Apply leaves
	for i := range m.leaves {
		leaf := &m.leaves[i]
		name := nameOf(leaf.fullSuffix)
		switch leaf.kind {
		case leafString:
			if s, ok := getString(name); ok {
				fv := v.FieldByIndex(leaf.index)
				if fv.CanSet() && (strategy == SetOverride || fv.IsZero()) {
					fv.SetString(s)
				}
			}
		case leafBool:
			if b, ok := getBool(name); ok {
				fv := v.FieldByIndex(leaf.index)
				if fv.CanSet() && (strategy == SetOverride || fv.IsZero()) {
					fv.SetBool(b)
				}
			}
		case leafInt:
			if n, ok := getInt(name); ok {
				fv := v.FieldByIndex(leaf.index)
				if fv.CanSet() && (strategy == SetOverride || fv.IsZero()) {
					fv.SetInt(n)
				}
			}
		case leafUint:
			if n, ok := getInt(name); ok && n >= 0 {
				fv := v.FieldByIndex(leaf.index)
				if fv.CanSet() && (strategy == SetOverride || fv.IsZero()) {
					fv.SetUint(uint64(n))
				}
			}
		case leafDuration:
			if d, ok := getDuration(name); ok {
				fv := v.FieldByIndex(leaf.index)
				if fv.CanSet() && (strategy == SetOverride || fv.IsZero()) {
					fv.SetInt(int64(d))
				}
			}
		case leafPtrString:
			if s, ok := getString(name); ok {
				fv := v.FieldByIndex(leaf.index)
				if fv.IsNil() {
					if fv.CanSet() {
						fv.Set(reflect.New(fv.Type().Elem()))
						fv.Elem().SetString(s)
					}
				} else if strategy == SetOverride {
					fv.Elem().SetString(s)
				}
			}
		case leafPtrBool:
			if b, ok := getBool(name); ok {
				fv := v.FieldByIndex(leaf.index)
				if fv.IsNil() {
					if fv.CanSet() {
						fv.Set(reflect.New(fv.Type().Elem()))
						fv.Elem().SetBool(b)
					}
				} else if strategy == SetOverride {
					fv.Elem().SetBool(b)
				}
			}
		case leafPtrInt:
			if n, ok := getInt(name); ok {
				fv := v.FieldByIndex(leaf.index)
				if fv.IsNil() {
					if fv.CanSet() {
						fv.Set(reflect.New(fv.Type().Elem()))
						fv.Elem().SetInt(n)
					}
				} else if strategy == SetOverride {
					fv.Elem().SetInt(n)
				}
			}
		case leafPtrUint:
			if n, ok := getInt(name); ok && n >= 0 {
				fv := v.FieldByIndex(leaf.index)
				if fv.IsNil() {
					if fv.CanSet() {
						fv.Set(reflect.New(fv.Type().Elem()))
						fv.Elem().SetUint(uint64(n))
					}
				} else if strategy == SetOverride {
					fv.Elem().SetUint(uint64(n))
				}
			}
		case leafPtrDuration:
			if d, ok := getDuration(name); ok {
				fv := v.FieldByIndex(leaf.index)
				if fv.IsNil() {
					if fv.CanSet() {
						fv.Set(reflect.New(fv.Type().Elem()))
						fv.Elem().SetInt(int64(d))
					}
				} else if strategy == SetOverride {
					fv.Elem().SetInt(int64(d))
				}
			}
		}
	}
}
