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

url="https://github.com/$REPO/releases/download/$tag/ranma_${os}_${arch}.tar.gz"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Baixando ranma $tag ($os/$arch)…"
curl -sSLf "$url" -o "$tmp/ranma.tar.gz"
tar -xzf "$tmp/ranma.tar.gz" -C "$tmp"

mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/ranma" "$INSTALL_DIR/ranma"

echo "✓ ranma $tag instalado em $INSTALL_DIR/ranma"
echo
echo "Próximos passos:"
echo "  1. garanta que $INSTALL_DIR está no PATH"
echo "  2. ranma shim install"
echo "  3. ranma doctor"
