package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/devlefel/ranma/internal/account"
	"github.com/devlefel/ranma/internal/importer"
	"github.com/devlefel/ranma/internal/provider"
)

// SecretReader asks the operator for one credential value.
type SecretReader func(prompt string) (string, error)

// PromptSecret reads a secret without echoing it when stdin is a terminal,
// and reads a piped line otherwise, so `… | ranma add …` also works.
func PromptSecret(prompt string) (string, error) {
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, prompt)
		raw, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		return string(raw), err
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return line, nil
}

// Add registers one account, either by importing the native CLI's credential
// or by asking for each field the provider's env templates reference.
func Add(w io.Writer, reg *provider.Registry, st *account.Store, providerName, accountName string,
	doImport bool, read SecretReader, homeDir string) error {

	p, ok := reg.ByName(providerName)
	if !ok {
		return unknownProvider(reg, providerName)
	}

	var fields map[string]string
	if doImport {
		imported, err := importer.Import(p.Import, homeDir, accountName)
		if err != nil {
			return err
		}
		fields = imported
	} else {
		fields = map[string]string{}
		for _, field := range p.Fields() {
			value, err := read(fmt.Sprintf("%s/%s — cole o valor de %s: ", providerName, accountName, field))
			if err != nil {
				return err
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return fmt.Errorf("ranma: %s vazio; nada foi salvo", field)
			}
			fields[field] = value
		}
	}

	for _, field := range p.Fields() {
		if strings.TrimSpace(fields[field]) == "" {
			return fmt.Errorf("ranma: campo %q ausente na credencial importada", field)
		}
	}

	st.Set(providerName, accountName, fields)
	if err := st.Save(); err != nil {
		return err
	}

	// A provider whose env templates reference no field is legal; do not
	// index Fields() blindly to render the confirmation.
	if names := p.Fields(); len(names) > 0 {
		fmt.Fprintf(w, "✓ %s/%s cadastrada (%s)\n",
			providerName, accountName, account.Mask(fields[names[0]]))
	} else {
		fmt.Fprintf(w, "✓ %s/%s cadastrada\n", providerName, accountName)
	}
	fmt.Fprintf(w, "  Declare no projeto com: ranma link %s %s\n", providerName, accountName)
	return nil
}

// Remove deletes one account.
func Remove(w io.Writer, st *account.Store, providerName, accountName string) error {
	if !st.Remove(providerName, accountName) {
		return fmt.Errorf("ranma: conta %s/%s não existe", providerName, accountName)
	}
	if err := st.Save(); err != nil {
		return err
	}
	fmt.Fprintf(w, "✓ %s/%s removida\n", providerName, accountName)
	return nil
}
