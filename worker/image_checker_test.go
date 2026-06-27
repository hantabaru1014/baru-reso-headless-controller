package worker

import (
	"context"
	"testing"

	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/stretchr/testify/assert"
)

// TestImageChecker_ObserverPanicDoesNotPropagate guards the panic
// boundary in callObserver — a buggy subscriber must not be able to
// kill ImageChecker's tick loop or short-circuit later subscribers.
func TestImageChecker_ObserverPanicDoesNotPropagate(t *testing.T) {
	t.Parallel()

	c := &ImageChecker{}

	called := 0
	panicking := NewImageObserver(func(_ context.Context, _ *port.ContainerImage) {
		panic("observer crashed")
	})
	good := NewImageObserver(func(_ context.Context, _ *port.ContainerImage) {
		called++
	})

	c.Subscribe(panicking)
	c.Subscribe(good)

	assert.NotPanics(t, func() {
		c.notify(t.Context(), &port.ContainerImage{Tag: "v1"})
	})

	assert.Equal(t, 1, called,
		"a non-panicking observer registered after a panicking one must still run")
}
