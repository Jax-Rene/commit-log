package handler

import (
	"testing"

	"github.com/commitlog/internal/service"
)

func TestBuildNavButtonsDashboardRequiresLogin(t *testing.T) {
	settings := service.SystemSettings{
		NavButtons: []service.NavButton{
			{Type: service.NavButtonTypeAbout},
			{Type: service.NavButtonTypeDashboard},
		},
	}

	buttons := buildNavButtons(settings, false)
	if len(buttons) != 1 {
		t.Fatalf("expected only about nav button when logged out, got %#v", buttons)
	}
	if buttons[0].Title != "About Me" || buttons[0].URL != "/about" {
		t.Fatalf("unexpected nav button when logged out: %#v", buttons[0])
	}

	buttons = buildNavButtons(settings, true)
	if len(buttons) != 2 {
		t.Fatalf("expected dashboard nav button when logged in, got %#v", buttons)
	}
	if buttons[1].Title != "Dashboard" || buttons[1].URL != "/admin/dashboard" {
		t.Fatalf("unexpected dashboard nav button: %#v", buttons[1])
	}
}
