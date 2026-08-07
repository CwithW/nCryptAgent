package ui

import (
	"fmt"
	"github.com/lxn/walk"
)

type GlobalConfView struct {
	*walk.GroupBox

	PinTimeoutEdit           *walk.LineEdit
	PinTimeoutOverrideSelect *walk.ComboBox
	PinTimeoutOverrideStart  *walk.PushButton
	PinTimeoutOverrideCancel *walk.PushButton
	PinTimeoutOverrideStatus *walk.TextLabel
	NotificationsEdit        *walk.CheckBox
}

func NewGlobalConfView(parent walk.Container) (*GlobalConfView, error) {
	var err error
	var disposables walk.Disposables
	defer disposables.Treat()

	gcv := new(GlobalConfView)

	if gcv.GroupBox, err = newPaddedGroupGrid(parent); err != nil {
		return nil, err
	}
	disposables.Add(gcv)

	gcv.SetTitle("Global Config:")

	layout := gcv.Layout().(*walk.GridLayout)
	layout.SetSpacing(6)
	layout.SetMargins(walk.Margins{10, 10, 10, 10})
	layout.SetColumnStretchFactor(1, 3)

	//Setup PIN Timeout
	pinTimeoutLabel, err := walk.NewTextLabel(gcv)
	if err != nil {
		return nil, err
	}
	layout.SetRange(pinTimeoutLabel, walk.Rectangle{0, 0, 1, 1})
	pinTimeoutLabel.SetTextAlignment(walk.AlignHNearVCenter)
	pinTimeoutLabel.SetText(fmt.Sprintf("&PIN Cache Duration:"))
	pinTimeoutLabel.SetToolTipText("Amount of time in seconds the PIN is cached for. Set a duration in seconds, 0 for no cache, or -1 for indefinite caching.")

	if gcv.PinTimeoutEdit, err = walk.NewLineEdit(gcv); err != nil {
		return nil, err
	}
	layout.SetRange(gcv.PinTimeoutEdit, walk.Rectangle{1, 0, 1, 1})
	gcv.PinTimeoutEdit.SetText("0")
	gcv.PinTimeoutEdit.SetAlignment(walk.AlignHFarVFar)

	// Setup the temporary permanent-cache override controls.
	overrideLabel, err := walk.NewTextLabel(gcv)
	if err != nil {
		return nil, err
	}
	layout.SetRange(overrideLabel, walk.Rectangle{0, 1, 1, 1})
	overrideLabel.SetTextAlignment(walk.AlignHNearVCenter)
	overrideLabel.SetText(fmt.Sprintf("&Temporary -1 Override:"))
	overrideLabel.SetToolTipText("Temporarily treat the configured PIN cache duration as -1. This setting is kept in memory only.")

	overrideControls, err := walk.NewComposite(gcv)
	if err != nil {
		return nil, err
	}
	layout.SetRange(overrideControls, walk.Rectangle{1, 1, 1, 1})
	overrideControls.SetLayout(walk.NewHBoxLayout())
	overrideControls.Layout().SetMargins(walk.Margins{})

	if gcv.PinTimeoutOverrideSelect, err = walk.NewDropDownBox(overrideControls); err != nil {
		return nil, err
	}
	gcv.PinTimeoutOverrideSelect.SetModel(pinTimeoutOverrideLabels)
	gcv.PinTimeoutOverrideSelect.SetCurrentIndex(0)
	gcv.PinTimeoutOverrideSelect.SetMinMaxSizePixels(walk.Size{Width: 130}, walk.Size{})

	if gcv.PinTimeoutOverrideStart, err = walk.NewPushButton(overrideControls); err != nil {
		return nil, err
	}
	gcv.PinTimeoutOverrideStart.SetText("&Start")

	if gcv.PinTimeoutOverrideCancel, err = walk.NewPushButton(overrideControls); err != nil {
		return nil, err
	}
	gcv.PinTimeoutOverrideCancel.SetText("S&top")
	gcv.PinTimeoutOverrideCancel.SetEnabled(false)

	overrideStatusLabel, err := walk.NewTextLabel(gcv)
	if err != nil {
		return nil, err
	}
	layout.SetRange(overrideStatusLabel, walk.Rectangle{0, 2, 1, 1})
	overrideStatusLabel.SetTextAlignment(walk.AlignHNearVCenter)
	overrideStatusLabel.SetText(fmt.Sprintf("Override Status:"))

	if gcv.PinTimeoutOverrideStatus, err = walk.NewTextLabel(gcv); err != nil {
		return nil, err
	}
	layout.SetRange(gcv.PinTimeoutOverrideStatus, walk.Rectangle{1, 2, 1, 1})
	gcv.PinTimeoutOverrideStatus.SetTextAlignment(walk.AlignHNearVCenter)
	gcv.PinTimeoutOverrideStatus.SetText("Inactive")

	//Setup the notifications checkbox
	notificationLabel, err := walk.NewTextLabel(gcv)
	if err != nil {
		return nil, err
	}
	layout.SetRange(notificationLabel, walk.Rectangle{0, 3, 1, 1})
	notificationLabel.SetTextAlignment(walk.AlignHNearVCenter)
	notificationLabel.SetText(fmt.Sprintf("&Show Notifications:"))
	notificationLabel.SetToolTipText("Show toast notifications when a key operation succeeds or fails.")

	if gcv.NotificationsEdit, err = walk.NewCheckBox(gcv); err != nil {
		return nil, err
	}
	layout.SetRange(gcv.NotificationsEdit, walk.Rectangle{1, 3, 1, 1})
	gcv.NotificationsEdit.SetChecked(true)
	gcv.NotificationsEdit.SetAlignment(walk.AlignHFarVFar)

	if err := walk.InitWrapperWindow(gcv); err != nil {
		return nil, err
	}
	gcv.SetDoubleBuffering(true)

	disposables.Spare()

	return gcv, nil
}

type PageantConfView struct {
	*walk.GroupBox

	ListenerEnabled *walk.CheckBox
}

func NewPageantConfView(parent walk.Container) (*PageantConfView, error) {
	var err error
	var disposables walk.Disposables
	defer disposables.Treat()

	cv := new(PageantConfView)

	if cv.GroupBox, err = newPaddedGroupGrid(parent); err != nil {
		return nil, err
	}
	disposables.Add(cv)

	cv.SetTitle("Pageant (PuTTY):")

	layout := cv.Layout().(*walk.GridLayout)
	layout.SetSpacing(6)
	layout.SetMargins(walk.Margins{10, 10, 10, 10})
	layout.SetColumnStretchFactor(1, 3)

	//Setup the name
	enabledLabel, err := walk.NewTextLabel(cv)
	if err != nil {
		return nil, err
	}
	layout.SetRange(enabledLabel, walk.Rectangle{0, 0, 1, 1})
	enabledLabel.SetTextAlignment(walk.AlignHNearVFar)
	enabledLabel.SetText(fmt.Sprintf("&Enabled:"))

	if cv.ListenerEnabled, err = walk.NewCheckBox(cv); err != nil {
		return nil, err
	}
	layout.SetRange(cv.ListenerEnabled, walk.Rectangle{1, 0, 1, 1})
	cv.ListenerEnabled.SetChecked(true)
	cv.ListenerEnabled.SetAlignment(walk.AlignHFarVFar)

	if err := walk.InitWrapperWindow(cv); err != nil {
		return nil, err
	}
	cv.SetDoubleBuffering(true)

	disposables.Spare()

	return cv, nil
}

type NamedPipeConfView struct {
	*walk.GroupBox

	ListenerEnabled *walk.CheckBox
}

func NewNamedPipeConfView(parent walk.Container) (*NamedPipeConfView, error) {
	var err error
	var disposables walk.Disposables
	defer disposables.Treat()

	cv := new(NamedPipeConfView)

	if cv.GroupBox, err = newPaddedGroupGrid(parent); err != nil {
		return nil, err
	}
	disposables.Add(cv)

	cv.SetTitle("Named Pipe (OpenSSH for Windows):")

	layout := cv.Layout().(*walk.GridLayout)
	layout.SetSpacing(6)
	layout.SetMargins(walk.Margins{10, 10, 10, 10})
	layout.SetColumnStretchFactor(1, 3)

	//Setup the name
	enabledLabel, err := walk.NewTextLabel(cv)
	if err != nil {
		return nil, err
	}
	layout.SetRange(enabledLabel, walk.Rectangle{0, 0, 1, 1})
	enabledLabel.SetTextAlignment(walk.AlignHNearVCenter)
	enabledLabel.SetText(fmt.Sprintf("&Enabled:"))

	if cv.ListenerEnabled, err = walk.NewCheckBox(cv); err != nil {
		return nil, err
	}
	layout.SetRange(cv.ListenerEnabled, walk.Rectangle{1, 0, 1, 1})
	cv.ListenerEnabled.SetChecked(true)
	cv.ListenerEnabled.SetAlignment(walk.AlignHFarVFar)

	if err := walk.InitWrapperWindow(cv); err != nil {
		return nil, err
	}
	cv.SetDoubleBuffering(true)

	disposables.Spare()

	return cv, nil
}

type VSockConfView struct {
	*walk.GroupBox

	ListenerEnabled *walk.CheckBox
	ShellScript     *walk.TextEdit
}

func NewVSockConfView(parent walk.Container) (*VSockConfView, error) {
	var err error
	var disposables walk.Disposables
	defer disposables.Treat()

	cv := new(VSockConfView)

	if cv.GroupBox, err = newPaddedGroupGrid(parent); err != nil {
		return nil, err
	}
	disposables.Add(cv)

	cv.SetTitle("WSL2 (Hyper-V Socket):")

	layout := cv.Layout().(*walk.GridLayout)
	layout.SetSpacing(6)
	layout.SetMargins(walk.Margins{10, 10, 10, 10})
	layout.SetColumnStretchFactor(1, 3)

	//Setup the name
	enabledLabel, err := walk.NewTextLabel(cv)
	if err != nil {
		return nil, err
	}
	layout.SetRange(enabledLabel, walk.Rectangle{0, 0, 1, 1})
	enabledLabel.SetTextAlignment(walk.AlignHNearVCenter)
	enabledLabel.SetText(fmt.Sprintf("&Enabled:"))

	if cv.ListenerEnabled, err = walk.NewCheckBox(cv); err != nil {
		return nil, err
	}
	layout.SetRange(cv.ListenerEnabled, walk.Rectangle{1, 0, 1, 1})
	cv.ListenerEnabled.SetChecked(true)
	cv.ListenerEnabled.SetAlignment(walk.AlignHFarVFar)

	// Shell script
	shellScriptLabel, err := walk.NewTextLabel(cv)
	if err != nil {
		return nil, err
	}
	layout.SetRange(shellScriptLabel, walk.Rectangle{0, 1, 1, 1})
	shellScriptLabel.SetTextAlignment(walk.AlignHNearVNear)
	shellScriptLabel.SetText(fmt.Sprintf("&Shell Script:"))

	if cv.ShellScript, err = walk.NewTextEdit(cv); err != nil {
		return nil, err
	}
	layout.SetRange(cv.ShellScript, walk.Rectangle{1, 1, 1, 1})
	cv.ShellScript.SetAlignment(walk.AlignHNearVNear)
	cv.ShellScript.SetText("")
	cv.ShellScript.SetReadOnly(true)
	cv.ShellScript.SetMinMaxSizePixels(walk.Size{Width: 700, Height: 300}, walk.Size{})
	cv.ShellScript.SetToolTipText("Place in .bashrc, .profile, or equivalent")

	if err := walk.InitWrapperWindow(cv); err != nil {
		return nil, err
	}
	cv.SetDoubleBuffering(true)

	disposables.Spare()

	return cv, nil
}

type CygwinConfView struct {
	*walk.GroupBox

	ListenerEnabled *walk.CheckBox
	ShellScript     *walk.TextEdit
}

func NewCygwinConfView(parent walk.Container) (*CygwinConfView, error) {
	var err error
	var disposables walk.Disposables
	defer disposables.Treat()

	cv := new(CygwinConfView)

	if cv.GroupBox, err = newPaddedGroupGrid(parent); err != nil {
		return nil, err
	}
	disposables.Add(cv)

	cv.SetTitle("Cygwin (GIT for Windows/MSYS/mingw):")

	layout := cv.Layout().(*walk.GridLayout)
	layout.SetSpacing(6)
	layout.SetMargins(walk.Margins{10, 10, 10, 10})
	layout.SetColumnStretchFactor(1, 3)

	//Setup the name
	enabledLabel, err := walk.NewTextLabel(cv)
	if err != nil {
		return nil, err
	}
	layout.SetRange(enabledLabel, walk.Rectangle{0, 0, 1, 1})
	enabledLabel.SetTextAlignment(walk.AlignHNearVCenter)
	enabledLabel.SetText(fmt.Sprintf("&Enabled:"))

	if cv.ListenerEnabled, err = walk.NewCheckBox(cv); err != nil {
		return nil, err
	}
	layout.SetRange(cv.ListenerEnabled, walk.Rectangle{1, 0, 1, 1})
	cv.ListenerEnabled.SetAlignment(walk.AlignHNearVCenter)
	cv.ListenerEnabled.SetChecked(true)
	cv.ListenerEnabled.SetAlignment(walk.AlignHFarVFar)

	// Shell script
	shellScriptLabel, err := walk.NewTextLabel(cv)
	if err != nil {
		return nil, err
	}
	layout.SetRange(shellScriptLabel, walk.Rectangle{0, 1, 1, 1})
	shellScriptLabel.SetTextAlignment(walk.AlignHNearVNear)
	shellScriptLabel.SetText(fmt.Sprintf("&Shell Script:"))

	if cv.ShellScript, err = walk.NewTextEdit(cv); err != nil {
		return nil, err
	}
	layout.SetRange(cv.ShellScript, walk.Rectangle{1, 1, 1, 1})
	cv.ShellScript.SetAlignment(walk.AlignHNearVNear)
	cv.ShellScript.SetText("")
	cv.ShellScript.SetReadOnly(true)
	cv.ShellScript.SetMinMaxSizePixels(walk.Size{Width: 700, Height: 100}, walk.Size{})
	cv.ShellScript.SetToolTipText("Place in .bashrc, .profile, or equivalent")

	if err := walk.InitWrapperWindow(cv); err != nil {
		return nil, err
	}
	cv.SetDoubleBuffering(true)

	disposables.Spare()

	return cv, nil
}
