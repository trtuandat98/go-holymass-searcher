package searcher

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func toMarkdown(t *testing.T, fragment string) string {
	t.Helper()
	// Wrap in a div so the fragment has a single container to render from.
	nodes, err := html.ParseFragment(strings.NewReader(fragment), &html.Node{
		Type: html.ElementNode, Data: "div", DataAtom: atom.Div,
	})
	if err != nil {
		t.Fatalf("parse fragment: %v", err)
	}
	var b strings.Builder
	for _, n := range nodes {
		renderMarkdown(&b, n)
	}
	return normalizeMarkdown(b.String())
}

func TestNodeToMarkdown(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"bold", `Thánh <b>Phê-rô</b> Tông Ðồ`, "Thánh **Phê-rô** Tông Ðồ"},
		{"italic", `bài đọc <i>Tin Mừng</i>`, "bài đọc *Tin Mừng*"},
		{"line break", `Bài đọc 1<br>Bài đọc 2`, "Bài đọc 1\nBài đọc 2"},
		{"paragraphs", `<p>Một</p><p>Hai</p>`, "Một\n\nHai"},
		{"link", `Xem <a href="/x">đây</a>`, "Xem [đây](/x)"},
		{"list", `<ul><li>Bài đọc 1</li><li>Bài đọc 2</li></ul>`, "- Bài đọc 1\n- Bài đọc 2"},
		{"collapses whitespace", "Lời  Chúa\n\t  hôm nay", "Lời Chúa hôm nay"},
		{"drops script", `Nội dung<script>evil()</script> lễ`, "Nội dung lễ"},
		{"nested inline", `<p>Thánh <b>Phê-rô</b> và <b>Phao-lô</b></p>`, "Thánh **Phê-rô** và **Phao-lô**"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := toMarkdown(t, tc.in); got != tc.want {
				t.Errorf("\n in:   %q\n got:  %q\n want: %q", tc.in, got, tc.want)
			}
		})
	}
}
