package app

import (
	tea "github.com/charmbracelet/bubbletea"
	overlay "github.com/rmhubbert/bubbletea-overlay"
	"mojosoftware.dev/lazykafka/internal/ui"
)

type ToastManager struct {
	toast     ui.Toast
	showToast bool
}

func NewToastManager() ToastManager {
	return ToastManager{
		showToast: false,
	}
}

func (tm *ToastManager) HandleMessage(msg tea.Msg) bool {
	if _, ok := msg.(ui.ToastTimeoutMsg); ok {
		tm.showToast = false
		return true
	}
	return false
}

func (tm *ToastManager) ShowSuccess(message string) tea.Cmd {
	tm.toast = ui.NewToast(message, ui.Success, ui.TopRight, 3)
	tm.showToast = true
	return tm.toast.Init()
}

func (tm *ToastManager) ShowError(message string) tea.Cmd {
	tm.toast = ui.NewToast(message, ui.Error, ui.TopRight, 5)
	tm.showToast = true
	return tm.toast.Init()
}

func (tm *ToastManager) ShowInfo(message string) tea.Cmd {
	tm.toast = ui.NewToast(message, ui.Info, ui.TopRight, 3)
	tm.showToast = true
	return tm.toast.Init()
}

func (tm *ToastManager) Wrap(content string) string {
	if !tm.showToast {
		return content
	}

	toastView := tm.toast.View()
	if toastView == "" {
		return content
	}

	var hPos, vPos overlay.Position
	switch tm.toast.Position {
	case ui.TopRight:
		hPos, vPos = overlay.Right, overlay.Top
	case ui.BottomRight:
		hPos, vPos = overlay.Right, overlay.Bottom
	case ui.TopLeft:
		hPos, vPos = overlay.Left, overlay.Top
	case ui.BottomLeft:
		hPos, vPos = overlay.Left, overlay.Bottom
	case ui.Center:
		hPos, vPos = overlay.Center, overlay.Center
	}
	return overlay.Composite(toastView, content, hPos, vPos, 2, 2)
}
