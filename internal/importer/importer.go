// Package importer lifts a credential out of a provider CLI's own config file.
package importer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/devlefel/ranma/internal/provider"
)

// Import reads the native CLI's config and returns ranma account fields.
// accountName selects which account to lift when the native format holds
// several (gh); formats that hold one (railway) ignore it.
func Import(spec *provider.ImportSpec, homeDir, accountName string) (map[string]string, error) {
	if spec == nil {
		return nil, errors.New("ranma: este provider não suporta --import; passe a credencial manualmente")
	}

	path := filepath.Join(homeDir, spec.Path)
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf(
			"ranma: %s não existe; faça o login no CLI nativo primeiro e rode o --import de novo", path)
	}
	if err != nil {
		return nil, fmt.Errorf("ranma: não consegui ler %s: %w", path, err)
	}

	switch spec.Kind {
	case "railway-config":
		return importRailway(raw, path)
	case "gh-hosts":
		return importGhHosts(raw, path, accountName)
	default:
		return nil, fmt.Errorf("ranma: import kind desconhecido %q", spec.Kind)
	}
}

func importRailway(raw []byte, path string) (map[string]string, error) {
	var cfg struct {
		User struct {
			Token string `json:"token"`
		} `json:"user"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("ranma: %s não é um JSON válido", path)
	}
	if cfg.User.Token == "" {
		return nil, fmt.Errorf("ranma: %s não tem user.token; rode `railway login` primeiro", path)
	}
	return map[string]string{"token": cfg.User.Token}, nil
}

func importGhHosts(raw []byte, path, accountName string) (map[string]string, error) {
	var hosts map[string]struct {
		Users map[string]struct {
			OAuthToken string `yaml:"oauth_token"`
		} `yaml:"users"`
	}
	if err := yaml.Unmarshal(raw, &hosts); err != nil {
		return nil, fmt.Errorf("ranma: %s não é um YAML válido", path)
	}

	host, ok := hosts["github.com"]
	if !ok || len(host.Users) == 0 {
		return nil, fmt.Errorf("ranma: %s não tem contas em github.com; rode `gh auth login`", path)
	}

	user, ok := host.Users[accountName]
	if !ok {
		return nil, fmt.Errorf(
			"ranma: conta %q não está em %s (disponíveis: %s)",
			accountName, path, strings.Join(slices.Sorted(maps.Keys(host.Users)), ", "))
	}
	if user.OAuthToken == "" {
		return nil, fmt.Errorf("ranma: conta %q em %s não tem oauth_token", accountName, path)
	}
	return map[string]string{"token": user.OAuthToken}, nil
}
