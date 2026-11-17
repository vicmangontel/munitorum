package picker

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/tui/view"
)

func runes(s string) tea.KeyMsg    { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
func key(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

var vendors = []Item{
	{ID: 1, Label: "Álvaro Pérez"},
	{ID: 2, Label: "Juan Gómez (Portones Acme)"},
	{ID: 3, Label: "Portones Acme"},
	{ID: 4, Label: "Basura"},
}

// host is a test form that owns a picker, as a real form would: it keeps what
// the picker reports and feeds every message back to the picker.
type host struct {
	picker    *Model
	chosen    *Item
	cancelled bool
}

func (h *host) send(msg tea.Msg) {
	switch m := msg.(type) {
	case SelectedMsg:
		h.chosen = &m.Item
		return
	case CancelledMsg:
		h.cancelled = true
		return
	case tea.BatchMsg:
		for _, c := range m {
			h.run(c)
		}
		return
	}
	h.run(h.picker.Update(msg))
}

func (h *host) run(cmd tea.Cmd) {
	if cmd != nil {
		if msg := cmd(); msg != nil {
			h.send(msg)
		}
	}
}

func newHost(items []Item, create Creator) *host { return &host{picker: New(items, create)} }

func TestPicker_ListsEveryItemBeforeTyping(t *testing.T) {
	// Arrange
	h := newHost(vendors, nil)

	// Act
	out := h.picker.View()

	// Assert
	for _, it := range vendors {
		if !strings.Contains(out, it.Label) {
			t.Errorf("view does not list %q:\n%s", it.Label, out)
		}
	}
}

func TestPicker_FiltersWhileTypingIgnoringCaseAndAccents(t *testing.T) {
	cases := []struct {
		name  string
		typed string
		want  []string
		not   []string
	}{
		{"part of a name", "acme", []string{"Juan Gómez (Portones Acme)", "Portones Acme"}, []string{"Álvaro", "Basura"}},
		{"upper case", "BASURA", []string{"Basura"}, []string{"Álvaro", "Acme"}},
		{"accent typed where the item has none", "gómez", []string{"Juan Gómez"}, []string{"Basura"}},
		{"no accent typed where the item has one", "alvaro", []string{"Álvaro Pérez"}, []string{"Basura"}},
		{"surrounding spaces", "  basura ", []string{"Basura"}, []string{"Álvaro"}},
		{"nothing matches", "zzz", []string{"Sin coincidencias"}, []string{"Basura", "Álvaro"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			h := newHost(vendors, nil)

			// Act
			h.send(runes(c.typed))

			// Assert
			out := h.picker.View()
			for _, w := range c.want {
				if !strings.Contains(out, w) {
					t.Errorf("view does not show %q:\n%s", w, out)
				}
			}
			for _, n := range c.not {
				if strings.Contains(out, n) {
					t.Errorf("view still shows %q:\n%s", n, out)
				}
			}
		})
	}
}

func TestPicker_FilterFollowsEveryKeystrokeIncludingBackspace(t *testing.T) {
	// Arrange
	h := newHost(vendors, nil)
	h.send(runes("bas"))
	narrowed := h.picker.View()

	// Act
	h.send(key(tea.KeyBackspace))
	h.send(key(tea.KeyBackspace))
	h.send(key(tea.KeyBackspace))

	// Assert
	if strings.Contains(narrowed, "Álvaro") || !strings.Contains(h.picker.View(), "Álvaro") {
		t.Fatalf("filter did not narrow and widen with the text:\nnarrowed:\n%s\nafter backspace:\n%s", narrowed, h.picker.View())
	}
}

func TestPicker_EnterSelectsTheHighlightedMatch(t *testing.T) {
	cases := []struct {
		name string
		keys []tea.KeyMsg
		want int64
	}{
		{"first item when nothing was typed", nil, 1},
		{"after moving down twice", []tea.KeyMsg{key(tea.KeyDown), key(tea.KeyDown)}, 3},
		{"moving past the end stays on the last", []tea.KeyMsg{key(tea.KeyDown), key(tea.KeyDown), key(tea.KeyDown), key(tea.KeyDown), key(tea.KeyDown)}, 4},
		{"moving up from the first stays on it", []tea.KeyMsg{key(tea.KeyUp)}, 1},
		{"first match after typing", []tea.KeyMsg{runes("acme")}, 2},
		{"typing resets the highlight to the first match", []tea.KeyMsg{key(tea.KeyDown), runes("acme")}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			h := newHost(vendors, nil)
			for _, k := range c.keys {
				h.send(k)
			}

			// Act
			h.send(key(tea.KeyEnter))

			// Assert
			if h.chosen == nil || h.chosen.ID != c.want {
				t.Fatalf("chosen = %v, want item %d", h.chosen, c.want)
			}
		})
	}
}

func TestPicker_EnterWithNoMatchSelectsNothing(t *testing.T) {
	// Arrange
	h := newHost(vendors, nil)
	h.send(runes("zzz"))

	// Act
	h.send(key(tea.KeyEnter))

	// Assert
	if h.chosen != nil || h.cancelled {
		t.Fatalf("chosen = %v, cancelled = %v; want neither", h.chosen, h.cancelled)
	}
}

func TestPicker_EscChoosesNoItem(t *testing.T) {
	// Arrange
	h := newHost(vendors, nil)

	// Act
	h.send(key(tea.KeyEsc))

	// Assert
	if !h.cancelled || h.chosen != nil {
		t.Fatalf("cancelled = %v, chosen = %v; want a cancellation and no item", h.cancelled, h.chosen)
	}
}

func TestPicker_ShowsOnlyAWindowOfMatchesThatFollowsTheHighlight(t *testing.T) {
	// Arrange
	var many []Item
	for i := 1; i <= 20; i++ {
		many = append(many, Item{ID: int64(i), Label: fmt.Sprintf("Proveedor %02d", i)})
	}
	h := newHost(many, nil)

	// Act
	before := h.picker.View()
	for range 15 {
		h.send(key(tea.KeyDown))
	}
	after := h.picker.View()

	// Assert
	if !strings.Contains(before, "Proveedor 01") || strings.Contains(before, "Proveedor 09") {
		t.Fatalf("the first view does not show exactly the first rows:\n%s", before)
	}
	if !strings.Contains(after, "> Proveedor 16") || strings.Contains(after, "Proveedor 01") {
		t.Fatalf("the window did not follow the highlight:\n%s", after)
	}
}

// created is a stand-in for the content that creates an item inline: it sends
// CreatedMsg for the item "Nueva SA" when enter is pressed.
type created struct{ keys []string }

func (c *created) Update(msg tea.Msg) tea.Cmd {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	c.keys = append(c.keys, k.String())
	if k.String() == "enter" {
		return func() tea.Msg { return CreatedMsg{Item: Item{ID: 99, Label: "Nueva SA"}} }
	}
	return nil
}
func (c *created) View() string          { return "formulario de alta" }
func (c *created) KeyHints() []view.Hint { return []view.Hint{{Key: "enter", Desc: "crear"}} }

func TestPicker_PlusCreatesInlineAndReturnsTheNewItemSelected(t *testing.T) {
	// Arrange
	form := &created{}
	h := newHost(vendors, func() view.Content { return form })

	// Act
	h.send(runes("+"))
	opened := h.picker.View()
	h.send(key(tea.KeyEnter))

	// Assert
	if !strings.Contains(opened, "formulario de alta") {
		t.Fatalf("+ did not open the creation content:\n%s", opened)
	}
	if h.chosen == nil || *h.chosen != (Item{ID: 99, Label: "Nueva SA"}) {
		t.Fatalf("chosen = %v, want the created item", h.chosen)
	}
}

func TestPicker_WhileCreatingKeysGoToTheCreationContentAndNotToTheFilter(t *testing.T) {
	// Arrange
	form := &created{}
	h := newHost(vendors, func() view.Content { return form })
	h.send(runes("+"))

	// Act
	h.send(runes("x"))
	h.send(key(tea.KeyDown))

	// Assert
	if got := strings.Join(form.keys, ","); got != "x,down" {
		t.Fatalf("creation content received %q, want x,down", got)
	}
}

func TestPicker_EscWhileCreatingReturnsToTheListWithoutCancelling(t *testing.T) {
	// Arrange
	h := newHost(vendors, func() view.Content { return &created{} })
	h.send(runes("+"))

	// Act
	h.send(key(tea.KeyEsc))

	// Assert
	if h.cancelled || h.chosen != nil || !strings.Contains(h.picker.View(), "Basura") {
		t.Fatalf("cancelled = %v, chosen = %v, view:\n%s; want the list back and nothing decided", h.cancelled, h.chosen, h.picker.View())
	}
}

func TestPicker_PlusIsTypedTextWhenTheHostCannotCreate(t *testing.T) {
	// Arrange
	h := newHost([]Item{{ID: 1, Label: "A+B"}, {ID: 2, Label: "Otro"}}, nil)

	// Act
	h.send(runes("+"))

	// Assert
	out := h.picker.View()
	if !strings.Contains(out, "A+B") || strings.Contains(out, "Otro") {
		t.Fatalf("+ was not treated as typed text:\n%s", out)
	}
}

func TestPicker_KeyHintsOfferCreateOnlyWhenTheHostCanCreate(t *testing.T) {
	cases := []struct {
		name   string
		create Creator
		want   bool
	}{
		{"host can create", func() view.Content { return &created{} }, true},
		{"host cannot create", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			p := New(vendors, c.create)

			// Act
			hints := p.KeyHints()

			// Assert
			has := false
			for _, h := range hints {
				has = has || h.Key == "+"
			}
			if has != c.want {
				t.Fatalf("hints = %v, + offered = %v, want %v", hints, has, c.want)
			}
		})
	}
}

func TestPicker_HandlesEscSoTheHostDoesNotCloseAroundIt(t *testing.T) {
	// Act
	p := New(vendors, nil)

	// Assert
	if !p.HandlesEscape() {
		t.Fatal("the picker must receive esc itself")
	}
}
