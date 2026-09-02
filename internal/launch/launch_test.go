package launch

import (
	"reflect"
	"runtime"
	"testing"
)

func TestSavedataArgIsAbsoluteAndPrefixed(t *testing.T) {
	got := SavedataArg("/home/x/.local/share/rimforge/profiles/vanilla")
	if got != "-savedatafolder=/home/x/.local/share/rimforge/profiles/vanilla" {
		t.Fatal(got)
	}
}

func TestPlatformCommand(t *testing.T) {
	switch runtime.GOOS {
	case "linux":
		program, args, err := command("/p/vanilla")
		if err != nil {
			t.Fatal(err)
		}
		if program != "steam" || !reflect.DeepEqual(args, []string{"-applaunch", "294100", "-savedatafolder=/p/vanilla"}) {
			t.Fatalf("got %s %v", program, args)
		}
	case "darwin":
		program, args, err := command("/p/vanilla")
		if err != nil {
			t.Fatal(err)
		}
		if program != "open" || args[0] != "-a" || args[1] != "Steam" || args[2] != "--args" {
			t.Fatalf("got %s %v", program, args)
		}
	default:
		t.Skip("windows command depends on a Steam install")
	}
}

func TestUnknownProfileDoesNotLaunch(t *testing.T) {
	t.Setenv("RIMFORGE_DATA_DIR", t.TempDir())
	if err := Profile("nope"); err == nil {
		t.Fatal("expected an error for an unknown profile")
	}
}
