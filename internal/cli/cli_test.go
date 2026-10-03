package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/edududs/whatsapp-extractor-go/internal/cli"
)

func TestOfflineCommands(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
		fail bool
	}{
		{nil, "Usage", false}, {[]string{"version"}, "test-version", false}, {[]string{"check"}, "configuration valid", false}, {[]string{"check", "-account", "invalid"}, "", true}, {[]string{"run", "extra"}, "", true}, {[]string{"unknown"}, "", true}, {[]string{"check", "-config", "/nonexistent/config.toml"}, "", true},
	} {
		var out, logs bytes.Buffer
		err := cli.Run(t.Context(), tc.args, &out, &logs, "test-version", nil)
		if (err != nil) != tc.fail {
			t.Fatalf("%v: %v", tc.args, err)
		}
		if !strings.Contains(out.String(), tc.want) {
			t.Fatalf("%v: %s", tc.args, out.String())
		}
	}
}
