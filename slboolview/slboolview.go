// Copyright (c) 2023, The Goki Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slboolview

import (
	"cogentcore.org/core/events"
	"cogentcore.org/core/gi"
	"cogentcore.org/core/giv"
	"cogentcore.org/core/laser"
	"github.com/emer/gosl/v2/slbool"
)

func init() {
	giv.AddValue(slbool.Bool(0), func() giv.Value {
		return &BoolValue{}
	})
}

// BoolValue presents a checkbox for a boolean
type BoolValue struct {
	giv.ValueBase[*gi.Switch]
}

func (v *BoolValue) Config() {
	v.Widget.OnFinal(events.Change, func(e events.Event) {
		v.SetValue(v.Widget.IsChecked())
	})
}

func (v *BoolValue) Update() {
	npv := laser.NonPtrValue(v.Value)
	sb, ok := npv.Interface().(slbool.Bool)
	if ok {
		v.Widget.SetChecked(sb.IsTrue())
	} else {
		sb, ok := npv.Interface().(*slbool.Bool)
		if ok {
			v.Widget.SetChecked(sb.IsTrue())
		}
	}
}
