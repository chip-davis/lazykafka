package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	overlay "github.com/rmhubbert/bubbletea-overlay"
)

type ToastManager struct {
	toast     Toast
	showToast bool
}

func NewToastManager() ToastManager {
	return ToastManager{
		showToast: false,
	}
}

func (tm *ToastManager) HandleMessage(msg tea.Msg) bool {
	if _, ok := msg.(ToastTimeoutMsg); ok {
		tm.showToast = false
		return true
	}
	return false
}

func (tm *ToastManager) ShowSuccess(message string) tea.Cmd {
	tm.toast = NewToast(message, Success, TopRight, 3)
	tm.showToast = true
	return tm.toast.Init()
}

func (tm *ToastManager) ShowError(message string) tea.Cmd {
	tm.toast = NewToast(message, Error, TopRight, 5)
	tm.showToast = true
	return tm.toast.Init()
}

func (tm *ToastManager) ShowInfo(message string) tea.Cmd {
	tm.toast = NewToast(message, Info, TopRight, 3)
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
	case TopRight:
		hPos, vPos = overlay.Right, overlay.Top
	case BottomRight:
		hPos, vPos = overlay.Right, overlay.Bottom
	case TopLeft:
		hPos, vPos = overlay.Left, overlay.Top
	case BottomLeft:
		hPos, vPos = overlay.Left, overlay.Bottom
	case Center:
		hPos, vPos = overlay.Center, overlay.Center
	}
	return overlay.Composite(toastView, content, hPos, vPos, 2, 2)
}
