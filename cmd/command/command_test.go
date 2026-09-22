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

func TestGenErrorCodesCommandIsRegistered(t *testing.T) {
	for _, child := range Cmd.Commands() {
		if child.Name() == "gen-error-codes" {
			return
		}
	}
	t.Fatal("command gen-error-codes is not registered")
}
