#!/bin/sh
# Instalador do ranma. Uso: curl -sSL <url>/install.sh | sh
set -eu

REPO="devlefel/ranma"
INSTALL_DIR="${RANMA_INSTALL_DIR:-$HOME/.local/bin}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "ranma: arquitetura não suportada: $arch" >&2; exit 1 ;;
esac
case "$os" in
  linux|darwin) ;;
  *) echo "ranma: sistema não suportado: $os" >&2; exit 1 ;;
esac

tag=$(curl -sSL "https://api.github.com/repos/$REPO/releases/latest" |
  sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)
if [ -z "$tag" ]; then
  echo "ranma: não consegui descobrir a última release" >&2
  exit 1
fi

archive="ranma_${os}_${arch}.tar.gz"
url="https://github.com/$REPO/releases/download/$tag/$archive"
checksums_url="https://github.com/$REPO/releases/download/$tag/checksums.txt"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Baixando ranma $tag ($os/$arch)…"
curl -sSLf "$url" -o "$tmp/$archive"
curl -sSLf "$checksums_url" -o "$tmp/checksums.txt"

# ranma manages credentials; installing over curl | sh without verifying the
# download is not acceptable for a tool with that job. .goreleaser.yaml
# already publishes checksums.txt alongside every release — use it.
if command -v sha256sum >/dev/null 2>&1; then
  checker="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
  checker="shasum -a 256"
else
  echo "ranma: não encontrei sha256sum nem shasum; não dá para verificar o download com segurança, abortando" >&2
  exit 1
fi

grep "$archive\$" "$tmp/checksums.txt" > "$tmp/checksums.want" || {
  echo "ranma: $archive não aparece em checksums.txt; abortando sem instalar" >&2
  exit 1
}
if ! (cd "$tmp" && $checker -c checksums.want) >/dev/null 2>&1; then
  echo "ranma: checksum do download não confere; abortando sem instalar" >&2
  exit 1
fi

tar -xzf "$tmp/$archive" -C "$tmp"

mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/ranma" "$INSTALL_DIR/ranma"

echo "✓ ranma $tag instalado em $INSTALL_DIR/ranma"
echo

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "⚠ $INSTALL_DIR não está no seu PATH; adicione antes de continuar" ;;
esac

echo "Próximos passos:"
echo "  1. garanta que $INSTALL_DIR está no PATH"
echo "  2. ranma shim install"
echo "  3. ranma doctor"
