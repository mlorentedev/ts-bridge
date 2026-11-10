package bootstrapssh

import (
	"reflect"
	"testing"
)

// TestStartWiresContainment pins the dependencies Start passes to start
// (REL-376): the containment hooks are what keep ssh from outliving
// ts-bridge, and every other test injects its own, so dropping them from
// the defaults would leave the whole suite green (#376).
func TestStartWiresContainment(t *testing.T) {
	deps := defaultDependencies()
	if deps.prepare == nil || reflect.ValueOf(deps.prepare).Pointer() != reflect.ValueOf(prepareContainment).Pointer() {
		t.Error("Start does not pass prepareContainment as prepare")
	}
	if deps.contain == nil || reflect.ValueOf(deps.contain).Pointer() != reflect.ValueOf(containProcess).Pointer() {
		t.Error("Start does not pass containProcess as contain")
	}
}
