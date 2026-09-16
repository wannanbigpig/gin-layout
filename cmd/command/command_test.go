package command

import "testing"

func TestGenOpenAPICommandIsRegistered(t *testing.T) {
	for _, child := range Cmd.Commands() {
		if child.Name() == "gen-openapi" {
			return
		}
	}
	t.Fatal("command gen-openapi is not registered")
}
