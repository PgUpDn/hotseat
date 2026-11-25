package desktop

import "testing"

func SetBeforeSwapStep(t *testing.T, fn func(step string) error) {
	t.Helper()
	old := beforeSwapStep
	beforeSwapStep = fn
	t.Cleanup(func() { beforeSwapStep = old })
}

func (d LinuxDriver) MainPIDs() ([]int, error) { return d.mainPIDs() }
