package ui

import (
	"fmt"
	"github.com/lxn/walk"
	"ncryptagent/keyman"
	"ncryptagent/keyman/listeners"
	"strconv"
	"time"
)

var pinTimeoutOverrideLabels = []string{
	"30 minutes",
	"1 hour",
	"2 hours",
	"6 hours",
	"12 hours",
	"24 hours",
	"This lifetime",
}

var pinTimeoutOverrideDurations = []time.Duration{
	30 * time.Minute,
	time.Hour,
	2 * time.Hour,
	6 * time.Hour,
	12 * time.Hour,
	24 * time.Hour,
}

type ConfPageView struct {
	*walk.ScrollView
	globalConfView    *GlobalConfView
	pageantConfView   *PageantConfView
	cygwinConfView    *CygwinConfView
	vsockConfView     *VSockConfView
	namedPipeConfView *NamedPipeConfView
	saveButton        *walk.PushButton
}

func NewConfPageView(parent walk.Container) (*ConfPageView, error) {
	var err error
	var disposables walk.Disposables
	defer disposables.Treat()

	cpv := new(ConfPageView)
	if cpv.ScrollView, err = walk.NewScrollView(parent); err != nil {
		return nil, err
	}
	disposables.Add(cpv)

	vlayout := walk.NewVBoxLayout()
	vlayout.SetMargins(walk.Margins{5, 0, 5, 0})
	cpv.SetLayout(vlayout)

	if cpv.globalConfView, err = NewGlobalConfView(cpv); err != nil {
		return nil, err
	}

	if cpv.pageantConfView, err = NewPageantConfView(cpv); err != nil {
		return nil, err
	}

	if cpv.namedPipeConfView, err = NewNamedPipeConfView(cpv); err != nil {
		return nil, err
	}

	if cpv.vsockConfView, err = NewVSockConfView(cpv); err != nil {
		return nil, err
	}

	if cpv.cygwinConfView, err = NewCygwinConfView(cpv); err != nil {
		return nil, err
	}

	if cpv.saveButton, err = walk.NewPushButton(cpv); err != nil {
		return nil, err
	}
	cpv.saveButton.SetText(fmt.Sprintf("&Save"))
	//dlg.saveButton.Clicked().Attach(dlg.onSaveButtonClicked)

	disposables.Spare()

	return cpv, nil
}

type ConfPage struct {
	*walk.TabPage
	keyManager   *keyman.KeyManager
	confPageView *ConfPageView

	globalConfView    *GlobalConfView
	pageantConfView   *PageantConfView
	cygwinConfView    *CygwinConfView
	vsockConfView     *VSockConfView
	namedPipeConfView *NamedPipeConfView
}

func NewConfPage(keyManager *keyman.KeyManager) (*ConfPage, error) {
	var err error
	var disposables walk.Disposables
	defer disposables.Treat()

	cp := new(ConfPage)

	cp.keyManager = keyManager

	if cp.TabPage, err = walk.NewTabPage(); err != nil {
		return nil, err
	}

	disposables.Add(cp)

	cp.SetTitle(fmt.Sprintf("Config"))
	vlayout := walk.NewVBoxLayout()
	vlayout.SetMargins(walk.Margins{5, 0, 5, 0})
	cp.SetLayout(vlayout)

	if cp.confPageView, err = NewConfPageView(cp); err != nil {
		return nil, err
	}

	cp.confPageView.saveButton.Clicked().Attach(cp.onSaveConfigClicked)
	cp.confPageView.globalConfView.PinTimeoutOverrideStart.Clicked().Attach(cp.onStartPinTimeoutOverrideClicked)
	cp.confPageView.globalConfView.PinTimeoutOverrideCancel.Clicked().Attach(cp.onCancelPinTimeoutOverrideClicked)
	cp.confPageView.VisibleChanged().Attach(cp.onTabSelected)

	disposables.Spare()

	return cp, nil
}

func (cp *ConfPage) onSaveConfigClicked() {
	intVal, err := strconv.Atoi(cp.confPageView.globalConfView.PinTimeoutEdit.Text())
	if err != nil {
		showError(fmt.Errorf("Invalid Pin Cache duration %s", cp.confPageView.globalConfView.PinTimeoutEdit.Text()), cp.Form())
		return
	}
	if err := cp.keyManager.SetPinTimeout(intVal); err != nil {
		showError(err, cp.Form())
		return
	}

	cp.keyManager.SetNotificationsEnabled(cp.confPageView.globalConfView.NotificationsEdit.Checked())
	cp.keyManager.EnableListener(listeners.TYPE_PAGEANT, cp.confPageView.pageantConfView.ListenerEnabled.Checked())
	cp.keyManager.EnableListener(listeners.TYPE_NAMED_PIPE, cp.confPageView.namedPipeConfView.ListenerEnabled.Checked())
	cp.keyManager.EnableListener(listeners.TYPE_VSOCK, cp.confPageView.vsockConfView.ListenerEnabled.Checked())
	cp.keyManager.EnableListener(listeners.TYPE_CYGWIN, cp.confPageView.cygwinConfView.ListenerEnabled.Checked())

	if err := cp.keyManager.SaveConfig(); err != nil {
		showError(fmt.Errorf("Unable to save configuration: %w", err), cp.Form())
		return
	}

	cp.onTabSelected() // refresh the view
}

func (cp *ConfPage) onStartPinTimeoutOverrideClicked() {
	index := cp.confPageView.globalConfView.PinTimeoutOverrideSelect.CurrentIndex()
	if index < 0 || index >= len(pinTimeoutOverrideLabels) {
		showError(fmt.Errorf("Invalid temporary PIN cache override selection"), cp.Form())
		return
	}

	if index == len(pinTimeoutOverrideDurations) {
		cp.keyManager.StartPinTimeoutOverrideForLifetime()
	} else if err := cp.keyManager.StartPinTimeoutOverride(pinTimeoutOverrideDurations[index]); err != nil {
		showError(err, cp.Form())
		return
	}

	cp.refreshPinTimeoutOverrideStatus()
	status := cp.keyManager.GetPinTimeoutOverrideStatus()
	if status.Active && !status.UntilRestart {
		go cp.refreshPinTimeoutOverrideStatusAt(status.ExpiresAt)
	}
}

func (cp *ConfPage) onCancelPinTimeoutOverrideClicked() {
	cp.keyManager.CancelPinTimeoutOverride()
	cp.refreshPinTimeoutOverrideStatus()
}

func (cp *ConfPage) refreshPinTimeoutOverrideStatusAt(expiresAt time.Time) {
	// Let the KeyManager expiry callback restore the configured timeout first.
	delay := time.Until(expiresAt) + 100*time.Millisecond
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		<-timer.C
	}
	cp.Synchronize(cp.refreshPinTimeoutOverrideStatus)
}

func (cp *ConfPage) refreshPinTimeoutOverrideStatus() {
	status := cp.keyManager.GetPinTimeoutOverrideStatus()
	statusLabel := cp.confPageView.globalConfView.PinTimeoutOverrideStatus
	cancelButton := cp.confPageView.globalConfView.PinTimeoutOverrideCancel

	cancelButton.SetEnabled(status.Active)
	if !status.Active {
		statusLabel.SetText("Inactive; using configured timeout")
		return
	}
	if status.UntilRestart {
		statusLabel.SetText("Active until nCryptAgent restarts")
		return
	}

	statusLabel.SetText(fmt.Sprintf("Active until %s", status.ExpiresAt.Format("2006-01-02 15:04:05")))
}

func (cp *ConfPage) onTabSelected() {
	if cp.confPageView.Visible() {
		cp.confPageView.vsockConfView.ShellScript.SetText(
			fmt.Sprintf(
				"# Ensure you have socat version >= 1.7.4 installed in your WSL2 environment\r\n"+
					"export SSH_AUTH_SOCK=/tmp/ssh-agent-hv.sock\r\n"+
					"ss -lnx | grep -q $SSH_AUTH_SOCK\r\nif [ $? -ne 0 ]; then\r\n"+
					"  rm -f $SSH_AUTH_SOCK\r\n"+
					"  (setsid -f nohup socat UNIX-LISTEN:$SSH_AUTH_SOCK,fork VSOCK-CONNECT:2:0x%x >/dev/null 2>&1)\r\n"+
					"fi\r\n", listeners.VSockServicePort))
		cp.confPageView.cygwinConfView.ShellScript.SetText(fmt.Sprintf("export SSH_AUTH_SOCK=\"%s\"", cp.keyManager.CygwinSocketLocation()))

		cp.confPageView.pageantConfView.ListenerEnabled.SetChecked(cp.keyManager.GetListenerEnabled(listeners.TYPE_PAGEANT))
		cp.confPageView.namedPipeConfView.ListenerEnabled.SetChecked(cp.keyManager.GetListenerEnabled(listeners.TYPE_NAMED_PIPE))
		cp.confPageView.vsockConfView.ListenerEnabled.SetChecked(cp.keyManager.GetListenerEnabled(listeners.TYPE_VSOCK))
		cp.confPageView.cygwinConfView.ListenerEnabled.SetChecked(cp.keyManager.GetListenerEnabled(listeners.TYPE_CYGWIN))
		cp.confPageView.globalConfView.PinTimeoutEdit.SetText(strconv.Itoa(cp.keyManager.GetPinTimeout()))
		cp.confPageView.globalConfView.NotificationsEdit.SetChecked(cp.keyManager.GetNotificationsEnabled())
		cp.refreshPinTimeoutOverrideStatus()
	}
}
