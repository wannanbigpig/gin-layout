package console

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"go.uber.org/zap"
)

// ConfirmOperation 确认操作；assumeYes=true 时跳过交互确认。
func ConfirmOperation(prompt string, assumeYes bool) bool {
	if assumeYes {
		return true
	}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print(prompt)

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			log.Logger.Error("Failed to read user input", zap.Error(err))
			_, _ = fmt.Fprintln(os.Stderr, "reading standard input:", err)
		}
		return false
	}

	input := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return input == "y" || input == "yes"
}
