#!/bin/sh

set -eu

# Parse command line arguments
VERSION=""
while [ $# -gt 0 ]; do
  case "$1" in
    -v|--version)
      VERSION="$2"
      shift 2
      ;;
    -h|--help)
      echo "Usage: $0 [options]"
      echo ""
      echo "Options:"
      echo "  -v, --version VERSION    Install specific version (e.g., 0.11.0 or 0.11.0-beta.11)"
      echo "  -h, --help               Show this help message"
      echo ""
      echo "Examples:"
      echo "  $0                       # Install latest stable version"
      echo "  $0 -v 0.11.0             # Install specific stable version"
      echo "  $0 -v 0.11.0-beta.11     # Install specific beta version"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      echo "Use -h or --help for usage information"
      exit 1
      ;;
  esac
done

os=$(uname -s)
arch=$(uname -m)
OS=${OS:-"${os}"}
ARCH=${ARCH:-"${arch}"}
OS_ARCH=""

BIN=${BIN:-overlock}

unsupported_arch() {
  local os=$1
  local arch=$2
  echo "overlock does not support $os / $arch at this time."
  exit 1
}

# If no version specified, get the latest stable release (excluding beta, alpha, rc versions)
if [ -z "$VERSION" ]; then
  VERSION=$(curl -sL "https://api.github.com/repos/web-seven/overlock/releases" | \
    grep '"tag_name":' | \
    sed -E 's/.*"tag_name": "([^"]+)".*/\1/' | \
    grep -v -E '(alpha|beta|rc)' | \
    head -n 1)
fi

case $OS in
  CYGWIN* | MINGW64* | Windows*)
    if [ $ARCH = "x86_64" ]
    then
      OS_ARCH=windows-amd64
      BIN=overlock.exe
    else
      unsupported_arch $OS $ARCH
    fi
    ;;
  Darwin)
    case $ARCH in
      x86_64|amd64)
        OS_ARCH=darwin-amd64
        ;;
      arm64)
        OS_ARCH=darwin-arm64
        ;;
      *)
        unsupported_arch $OS $ARCH
        ;;
    esac
    ;;
  Linux)
    case $ARCH in
      x86_64|amd64)
        OS_ARCH=linux-amd64
        ;;
      arm64|aarch64)
        OS_ARCH=linux-arm64
        ;;
      *)
        unsupported_arch $OS $ARCH
        ;;
    esac
    ;;
  *)
    unsupported_arch $OS $ARCH
    ;;
esac

url=https://github.com/web-seven/overlock/releases/download/${VERSION}/overlock-${VERSION}-${OS_ARCH}.tar.gz
if ! curl -sfLo overlock.tar.gz "${url}"; then
  echo "Failed to download Overlock CLI. Please make sure version ${VERSION} exists on OS ${OS} and ${OS_ARCH} architecture."
  exit 1
fi

tar -xf overlock.tar.gz
rm overlock.tar.gz
chmod +x overlock

echo "Overlock CLI ${VERSION} for ${OS} ${ARCH} downloaded successfully!"
echo "Run the following commands to finish installing it:"
echo 
echo sudo mv overlock /usr/local/bin
echo overlock --version
echo
echo "Visit https://github.com/web-seven/overlock to get started. 🚀"
echo "Have a nice day! 👋\n"