package k8s

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

// Eventf records an event on obj and is a no-op when rec is nil, so components built without a recorder stay silent.
func Eventf(rec record.EventRecorder, obj runtime.Object, eventType, reason, format string, args ...any) {
	if rec != nil {
		rec.Eventf(obj, eventType, reason, format, args...)
	}
}
