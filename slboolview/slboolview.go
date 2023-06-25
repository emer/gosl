// Copyright (c) 2023, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slboolview

import (
	"reflect"

	"github.com/goki/gi/gi"
	"github.com/goki/gi/giv"
	"github.com/goki/gosl/slbool"
	"github.com/goki/ki/ki"
	"github.com/goki/ki/kit"
)

func init() {
	var bi slbool.Bool
	giv.ValueViewMapAdd(kit.LongTypeName(reflect.TypeOf(bi)), func() giv.ValueView {
		vv := &BoolValueView{}
		ki.InitNode(vv)
		return vv
	})
}

// BoolValueView presents a checkbox for a boolean
type BoolValueView struct {
	giv.ValueViewBase
}

var KiT_BoolValueView = kit.Types.AddType(&BoolValueView{}, nil)

func (vv *BoolValueView) WidgetType() reflect.Type {
	vv.WidgetTyp = gi.KiT_CheckBox
	return vv.WidgetTyp
}

func (vv *BoolValueView) UpdateWidget() {
	if vv.Widget == nil {
		return
	}
	cb := vv.Widget.(*gi.CheckBox)
	npv := kit.NonPtrValue(vv.Value)
	sb, ok := npv.Interface().(slbool.Bool)
	if ok {
		cb.SetChecked(sb.IsTrue())
	} else {
		sb, ok := npv.Interface().(*slbool.Bool)
		if ok {
			cb.SetChecked(sb.IsTrue())
		}
	}
}

func (vv *BoolValueView) ConfigWidget(widg gi.Node2D) {
	vv.Widget = widg
	vv.StdConfigWidget(widg)
	cb := vv.Widget.(*gi.CheckBox)
	cb.Tooltip, _ = vv.Tag("desc")
	cb.SetInactiveState(vv.This().(giv.ValueView).IsInactive())
	cb.ButtonSig.ConnectOnly(vv.This(), func(recv, send ki.Ki, sig int64, data any) {
		if sig == int64(gi.ButtonToggled) {
			vvv, _ := recv.Embed(KiT_BoolValueView).(*BoolValueView)
			cbb := vvv.Widget.(*gi.CheckBox)
			if vvv.SetValue(cbb.IsChecked()) {
				vvv.UpdateWidget() // always update after setting value..
			}
		}
	})
	vv.UpdateWidget()
}
