package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
	"github.com/vicmangontel/munitorum/internal/tui/wizard"
)

func settings() app.Settings {
	return app.Settings{
		CommunityName: "Coto Los Pinos",
		Accounts: []app.Account{
			{Name: "Efectivo", Kind: app.KindCash, OpeningBalance: domain.Money(750000)},
			{Name: "Banco (NU)", Kind: app.KindBank, OpeningBalance: domain.Money(120050)},
		},
	}
}

func TestShell_SummaryPanelShowsTheCotoNameAndTheOpeningBalances(t *testing.T) {
	// Arrange
	m := sized(100, 30).WithSettings(settings())

	// Act
	out := m.View()

	// Assert
	for _, want := range []string{"Coto Los Pinos", "Efectivo", "$7,500.00", "Banco (NU)", "$1,200.50", "$8,700.50"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary panel does not show %q:\n%s", want, out)
		}
	}
}

func TestRoot_ShowsTheShellOfAnExistingCoto(t *testing.T) {
	// Arrange
	var model tea.Model = NewShell(nil, settings())

	// Act
	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	// Assert
	if model.(Root).InWizard() || !strings.Contains(model.View(), "Coto Los Pinos") {
		t.Fatalf("root is not the shell of the coto:\n%s", model.View())
	}
}

func TestRoot_ReplacesTheWizardWithTheShellWhenSetupIsDone(t *testing.T) {
	// Arrange
	today := time.Date(2026, time.November, 13, 0, 0, 0, 0, time.Local)
	var model tea.Model = NewWizard(nil, today)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	inWizard := model.View()

	// Act
	model, _ = model.Update(wizard.DoneMsg{Settings: settings()})

	// Assert
	if !strings.Contains(inWizard, "Configuración inicial") {
		t.Fatalf("root did not start on the wizard:\n%s", inWizard)
	}
	out := model.View()
	if model.(Root).InWizard() || strings.Contains(out, "Configuración inicial") ||
		!strings.Contains(out, "Coto Los Pinos") || !strings.Contains(out, "$7,500.00") {
		t.Fatalf("root did not open the shell with the coto:\n%s", out)
	}
}
