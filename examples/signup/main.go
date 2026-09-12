// Command signup is an interactive signup helper for the QRocodile QR Code API.
//
//	go run ./examples/signup [email] [code]
//	QR_API_URL=http://localhost:3002 go run ./examples/signup you@example.com
//
// Flow: register an email → a 6-digit code arrives in your inbox → paste it here → the command
// prints your key to stdout (so it's pipeable: `signup you@example.com > key.txt`). All prompts
// and messages go to stderr.
//
// Non-interactive: pass the email as arg 1 and, once you have it, the code as arg 2.
//
// The API emails a code rather than a link because mail-security appliances prefetch links and
// burn single-use tokens before a human ever clicks.
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/qrocodile-io/qrocodile-api-go"
)

var nonDigit = regexp.MustCompile(`\D`)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	reader := bufio.NewReader(os.Stdin)
	ask := func(prompt string) string {
		fmt.Fprint(os.Stderr, prompt)
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line)
	}

	email := argOrEmpty(1)
	if email == "" {
		email = ask("Email: ")
	}
	if email == "" {
		return fmt.Errorf("an email address is required. Usage: signup <email> [code]")
	}

	baseURL := os.Getenv("QR_API_URL")
	if baseURL == "" {
		baseURL = qrocodile.DefaultBaseURL
	}
	qr := qrocodile.NewClient("", qrocodile.WithBaseURL(baseURL))
	ctx := context.Background()

	fmt.Fprintf(os.Stderr, "Registering %s at %s …\n", email, baseURL)
	if _, err := qr.RegisterKey(ctx, email, ""); err != nil {
		return fmt.Errorf("✗ Registration failed: %w", err)
	}
	fmt.Fprintln(os.Stderr, "✓ Email sent. It carries a 6-digit code — enter it here to reveal your API key.")

	code := codeFrom(argOrEmpty(2))
	if code == "" {
		code = codeFrom(ask("6-digit code: "))
	}
	if len(code) != 6 {
		return fmt.Errorf("a 6-digit code is required. Re-run with the code once the email arrives")
	}

	result, err := qr.ConfirmKey(ctx, code, qrocodile.ConfirmKeyIdentifier{Email: email})
	if err != nil {
		fmt.Fprintln(os.Stderr, "  Five wrong codes burn the pending signup — register again to get a new one.")
		return fmt.Errorf("✗ Confirmation failed: %w", err)
	}

	if result.Replaced {
		fmt.Fprintln(os.Stderr, "\n! This replaced an existing key for that address — the old one no longer works.")
	}
	fmt.Fprintln(os.Stderr, "\n✓ Your API key (store it now — it is not shown again):")
	fmt.Println(result.ApiKey) // stdout only, so it can be piped/captured
	return nil
}

func argOrEmpty(i int) string {
	if len(os.Args) > i {
		return os.Args[i]
	}
	return ""
}

// codeFrom accepts the code on its own, or pasted out of the email's verify link.
func codeFrom(input string) string {
	digits := nonDigit.ReplaceAllString(input, "")
	if len(digits) > 6 {
		digits = digits[:6]
	}
	return digits
}
