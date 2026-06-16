package docker

import "testing"

func TestMonitorStopIsIdempotent(t *testing.T) {
	monitor := &Monitor{
		stopCh: make(chan struct{}),
	}

	monitor.Stop()
	monitor.Stop()
}

func TestGetAppNameFromLabelsUsesSanitizedDefault(t *testing.T) {
	got := getAppNameFromLabels(map[string]string{}, "my_app")
	want := "watchcow.my-app"
	if got != want {
		t.Errorf("getAppNameFromLabels() = %q, want %q", got, want)
	}
}

func TestGetAppNameFromLabelsPreservesExplicitLabel(t *testing.T) {
	got := getAppNameFromLabels(map[string]string{
		"watchcow.appname": "custom.app",
	}, "my_app")
	want := "custom.app"
	if got != want {
		t.Errorf("getAppNameFromLabels() = %q, want %q", got, want)
	}
}
