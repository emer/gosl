// Copyright (c) 2023, The Goki Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slboolview

import (
	"goki.dev/gi/v2/gi"
	"goki.dev/gi/v2/giv"
	"goki.dev/goosi/events"
	"goki.dev/gosl/v2/slbool"
	"goki.dev/gti"
	"goki.dev/laser"
)

func init() {
	giv.ValueMapAdd(slbool.Bool(0), func() giv.Value {
		return &BoolValue{}
	})
}

// BoolValue presents a checkbox for a boolean
type BoolValue struct {
	giv.ValueBase
}

func (vv *BoolValue) WidgetType() *gti.Type {
	vv.WidgetTyp = gi.SwitchType
	return vv.WidgetTyp
}

func (vv *BoolValue) UpdateWidget() {
	if vv.Widget == nil {
		return
	}
	sw := vv.Widget.(*gi.Switch)
	npv := laser.NonPtrValue(vv.Value)
	sb, ok := npv.Interface().(slbool.Bool)
	if ok {
		sw.SetChecked(sb.IsTrue())
	} else {
		sb, ok := npv.Interface().(*slbool.Bool)
		if ok {
			sw.SetChecked(sb.IsTrue())
		}
	}
}

func (vv *BoolValue) ConfigWidget(w gi.Widget) {
	if vv.Widget == w {
		vv.UpdateWidget()
		return
	}
	vv.Widget = w
	vv.StdConfigWidget(w)
	sw := vv.Widget.(*gi.Switch)
	sw.OnLast(events.Change, func(e events.Event) {
		vv.SetValue(sw.IsChecked())
	})
	vv.UpdateWidget()
}
