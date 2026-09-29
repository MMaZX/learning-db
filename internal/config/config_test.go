package config

import "testing"

func TestGetEnvBool(t *testing.T) {
	cases := map[string]bool{"on": true, "TRUE": true, "1": true, "off": false, "false": false, "0": false, "": false}
	for v, want := range cases {
		t.Setenv("MCP_PASSIVE_CAPTURE", v)
		got, err := getEnvBool("MCP_PASSIVE_CAPTURE", false)
		if err != nil || got != want {
			t.Errorf("%q: got %v err %v, want %v", v, got, err, want)
		}
	}
	t.Setenv("MCP_PASSIVE_CAPTURE", "quizas")
	if _, err := getEnvBool("MCP_PASSIVE_CAPTURE", false); err == nil {
		t.Error("valor inválido debe fallar")
	}
}
