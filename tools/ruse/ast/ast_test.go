// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package ast

import (
	"testing"
)

var comments = []struct {
	list []string
	text string
}{
	{[]string{";"}, ""},
	{[]string{";   "}, ""},
	{[]string{";", ";", ";   "}, ""},
	{[]string{"; foo   "}, "foo\n"},
	{[]string{";", ";", "; foo"}, "foo\n"},
	{[]string{"; foo  bar  "}, "foo  bar\n"},
	{[]string{"; foo", "; bar"}, "foo\nbar\n"},
	{[]string{"; foo", ";", ";", ";", "; bar"}, "foo\n\nbar\n"},
	{[]string{"; foo", "; bar"}, "foo\nbar\n"},
	{[]string{";", ";", ";", "; foo", ";", ";", ";"}, "foo\n"},
}

func TestCommentText(t *testing.T) {
	for i, c := range comments {
		list := make([]*Comment, len(c.list))
		for i, s := range c.list {
			list[i] = &Comment{Text: s}
		}

		text := (&CommentGroup{list}).Text()
		if text != c.text {
			t.Errorf("case %d: got %q; expected %q", i, text, c.text)
		}
	}
}

func TestExpr_Print(t *testing.T) {
	tests := []struct {
		Expr Expression
		Want string
	}{
		{
			Expr: &BadExpression{},
			Want: "<bad expr>",
		},
		{
			Expr: &Identifier{Name: "bar"},
			Want: "bar",
		},
		{
			Expr: &Literal{Value: "123.4"},
			Want: "123.4",
		},
		{
			Expr: &List{Elements: []Expression{
				&Identifier{Name: "+"},
				&Literal{Value: "1"},
				&Literal{Value: "2"},
			}},
			Want: "(+ 1 2)",
		},
	}

	for _, test := range tests {
		got := test.Expr.Print()
		if got != test.Want {
			t.Errorf("%#v.Print():\nGot:  %s\nWant: %s", test.Expr, got, test.Want)
		}
	}
}
