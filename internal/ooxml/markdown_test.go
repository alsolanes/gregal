package ooxml

import (
	"reflect"
	"strings"
	"testing"
)

func TestFormatAmpliatDelMarkdown(t *testing.T) {
	cases := []struct {
		name, md string
		want     []Inline
	}{
		{"color", "a [b]{color=#c0392b} c", []Inline{{Text: "a "}, {Text: "b", Color: "c0392b"}, {Text: " c"}}},
		{"bg", "[b]{bg=#fff3a3}", []Inline{{Text: "b", Bg: "fff3a3"}}},
		{"font", `[b c]{font="Times New Roman"}`, []Inline{{Text: "b c", Font: "Times New Roman"}}},
		{"size", "[b]{size=14}", []Inline{{Text: "b", Size: 14}}},
		{"half size", "[b]{size=10.5}", []Inline{{Text: "b", Size: 10.5}}},
		{"underline", "a ++b++ c", []Inline{{Text: "a "}, {Text: "b", Underline: true}, {Text: " c"}}},
		{"all four", `[b]{color=#c0392b bg=#fff3a3 font="Georgia" size=14}`,
			[]Inline{{Text: "b", Color: "c0392b", Bg: "fff3a3", Font: "Georgia", Size: 14}}},
		{"bold inside", "[a **b**]{color=#112233}", []Inline{{Text: "a ", Color: "112233"}, {Text: "b", Bold: true, Color: "112233"}}},
		{"inside italic", "*a [b]{size=9}*", []Inline{{Text: "a ", Italic: true}, {Text: "b", Italic: true, Size: 9}}},
		{"nested spans", "[a [b]{color=#000000}]{color=#ffffff size=8}",
			[]Inline{{Text: "a ", Color: "ffffff", Size: 8}, {Text: "b", Color: "000000", Size: 8}}},
		{"underline in span", "[++a++ b]{bg=#ffff00}", []Inline{{Text: "a", Underline: true, Bg: "ffff00"}, {Text: " b", Bg: "ffff00"}}},
		{"code untouched", "`[a]{size=9} ++b++`", []Inline{{Text: "[a]{size=9} ++b++", Code: true}}},
		{"unknown key", "[a]{weight=bold}", []Inline{{Text: "[a]{weight=bold}"}}},
		{"bad colour", "[a]{color=red}", []Inline{{Text: "[a]{color=red}"}}},
		{"short colour", "[a]{color=#fff}", []Inline{{Text: "[a]{color=#fff}"}}},
		{"bad size", "[a]{size=big}", []Inline{{Text: "[a]{size=big}"}}},
		{"zero size", "[a]{size=0}", []Inline{{Text: "[a]{size=0}"}}},
		{"unquoted font", "[a]{font=Georgia}", []Inline{{Text: "[a]{font=Georgia}"}}},
		{"repeated key", "[a]{size=9 size=10}", []Inline{{Text: "[a]{size=9 size=10}"}}},
		{"empty attrs", "[a]{}", []Inline{{Text: "[a]{}"}}},
		{"one good one bad", "[a]{size=9 color=x}", []Inline{{Text: "[a]{size=9 color=x}"}}},
		{"lone underline", "a ++ b", []Inline{{Text: "a ++ b"}}},
		{"plain brackets", "[a] and {b}", []Inline{{Text: "[a] and {b}"}}},
	}
	for _, c := range cases {
		bs := ParseMarkdown(c.md)
		if len(bs) != 1 || !reflect.DeepEqual(bs[0].Inlines, c.want) {
			t.Errorf("%s: %q\n got %+v\nwant %+v", c.name, c.md, bs, c.want)
		}
	}
}

func TestAlineacioDeBloc(t *testing.T) {
	bs := ParseMarkdown("# Title\n{align=center}\n\nText one\nmore\n{align=justify}\n\nPlain\n\n> Quote\n{align=right}\n\n- item\n{align=left}")
	got := []string{}
	for _, b := range bs {
		got = append(got, InlinesText(b.Inlines)+"|"+b.ParaAlign)
	}
	want := []string{"Title|center", "Text one more|justify", "Plain|", "Quote|right", "item|left"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
	for _, md := range []string{"Text\n{align=middle}", "Text\n\n{align=center}", "Text\n {align=center}"} {
		var sb strings.Builder
		for _, b := range ParseMarkdown(md) {
			if b.ParaAlign != "" {
				t.Errorf("%q: alignment %q", md, b.ParaAlign)
			}
			sb.WriteString(InlinesText(b.Inlines))
		}
		if !strings.Contains(sb.String(), "{align=") {
			t.Errorf("%q: the line was dropped: %q", md, sb.String())
		}
	}
	bs = ParseMarkdown("```\ncode\n{align=center}\n```")
	if len(bs) != 1 || bs[0].Text != "code\n{align=center}" {
		t.Fatalf("code block: %+v", bs)
	}
}
