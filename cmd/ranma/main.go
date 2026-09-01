// Command ranma keeps provider CLI accounts scoped to the project directory.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/devlefel/ranma/internal/account"
	"github.com/devlefel/ranma/internal/cli"
	"github.com/devlefel/ranma/internal/paths"
	"github.com/devlefel/ranma/internal/provider"
	"github.com/devlefel/ranma/internal/resolve"
	"github.com/devlefel/ranma/internal/runner"
)

const usage = `ranma — a conta certa de CLI para cada projeto

Uso:
  ranma exec <bin> [args...]      executa resolvendo a conta do diretório
  ranma ls [provider]             lista contas cadastradas
  ranma whoami                    mostra o que este diretório resolve
  ranma link <provider> <conta>   declara a conta no .ranma.toml
  ranma add [--import] <provider> <conta>  cadastra credencial
  ranma rm <provider> <conta>     remove credencial
  ranma shim install|uninstall    instala os interceptadores no PATH
  ranma hook install|uninstall    registra o hook do Claude Code
  ranma doctor [--verify]         diagnostica a instalação
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "exec":
		err = cmdExec(os.Args[2:])
	case "ls":
		err = cmdLs(os.Args[2:])
	case "whoami":
		err = cmdWhoami()
	case "link":
		err = cmdLink(os.Args[2:])
	case "add":
		err = cmdAdd(os.Args[2:])
	case "rm":
		err = cmdRm(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "ranma: subcomando desconhecido %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// load builds the registry and the account store from the user's config.
func load() (*provider.Registry, *account.Store, error) {
	reg, err := provider.Load(paths.ProvidersFile())
	if err != nil {
		return nil, nil, err
	}
	st, err := account.Load(paths.AccountsFile())
	if err != nil {
		return nil, nil, err
	}
	return reg, st, nil
}

func cmdExec(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("ranma exec: falta o executável")
	}
	bin, rest := args[0], args[1:]

	reg, st, err := load()
	if err != nil {
		return err
	}

	shimDir := paths.ShimDir()
	realBin, err := runner.RealBinary(bin, os.Getenv("PATH"), shimDir)
	if err != nil {
		return err
	}

	p, known := reg.ByBin(bin)
	if !known || p.IsPassthrough(rest) {
		return runner.Passthrough(bin, realBin, rest)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	res, err := resolve.Resolve(p, st, cwd)
	if err != nil {
		return err
	}
	return runner.Run(res, realBin, rest)
}

func cmdLs(args []string) error {
	reg, st, err := load()
	if err != nil {
		return err
	}
	filter := ""
	if len(args) > 0 {
		filter = args[0]
	}
	return cli.List(os.Stdout, reg, st, filter)
}

func cmdWhoami() error {
	reg, st, err := load()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	return cli.Whoami(os.Stdout, reg, st, cwd)
}

func cmdLink(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("uso: ranma link <provider> <conta>")
	}
	reg, st, err := load()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	return cli.Link(os.Stdout, reg, st, cwd, args[0], args[1])
}

func cmdAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // the flag package prints English usage; main reports the error
	doImport := fs.Bool("import", false, "importa a credencial ativa do CLI nativo")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("uso: ranma add [--import] <provider> <conta>")
	}
	reg, st, err := load()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return cli.Add(os.Stdout, reg, st, fs.Arg(0), fs.Arg(1), *doImport, cli.PromptSecret, home)
}

func cmdRm(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("uso: ranma rm <provider> <conta>")
	}
	_, st, err := load()
	if err != nil {
		return err
	}
	return cli.Remove(os.Stdout, st, args[0], args[1])
}
