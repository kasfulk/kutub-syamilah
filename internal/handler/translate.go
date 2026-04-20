package handler

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// translateText calls the 'trans' (translate-shell) utility to translate text.
// targetLang should be "id", "en", etc.
func translateText(ctx context.Context, text, targetLang string) (string, error) {
	if text == "" {
		return "", nil
	}

	// Use the -b (brief) flag for just the translation output.
	// Format is :targetLang
	cmd := exec.CommandContext(ctx, "trans", "-b", ":"+targetLang, text)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("trans failed: %w (output: %s)", err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}
