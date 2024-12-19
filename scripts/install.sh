#!/bin/sh

set -eu

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

VERSION=$(curl --silent "https://api.github.com/repos/web-seven/overlock/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

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