package searcher

import (
	"testing"
	"time"
)

func TestMassString(t *testing.T) {
	d := time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)

	t.Run("full", func(t *testing.T) {
		m := Mass{
			Date:    d,
			Weekday: "Chủ Nhật",
			Name:    "Chúa Hiển Linh",
			Season:  "Mùa Giáng Sinh",
			Feast:   "Lễ trọng",
		}
		want := "📅 Chủ Nhật, 04/01/2026\n**Chúa Hiển Linh**\n_Lễ trọng · Mùa Giáng Sinh_"
		if got := m.String(); got != want {
			t.Errorf("\n got:  %q\n want: %q", got, want)
		}
	})

	t.Run("with description", func(t *testing.T) {
		m := Mass{Date: d, Weekday: "Thứ Năm", Name: "Đức Maria Mẹ Thiên Chúa",
			Feast: "Lễ trọng", Description: "Ds 6:22-27; Lc 2:16-21"}
		want := "📅 Thứ Năm, 04/01/2026\n**Đức Maria Mẹ Thiên Chúa**\n_Lễ trọng_\n\nDs 6:22-27; Lc 2:16-21"
		if got := m.String(); got != want {
			t.Errorf("\n got:  %q\n want: %q", got, want)
		}
	})

	t.Run("feria minimal", func(t *testing.T) {
		m := Mass{Date: d, Weekday: "Thứ Hai", Name: "Thứ Hai tuần I"}
		want := "📅 Thứ Hai, 04/01/2026\n**Thứ Hai tuần I**"
		if got := m.String(); got != want {
			t.Errorf("\n got:  %q\n want: %q", got, want)
		}
	})
}
