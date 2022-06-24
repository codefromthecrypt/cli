#!/usr/bin/env bash

# apex CLI location
: ${APEX_INSTALL_DIR:="/usr/local/bin"}

# sudo is required to copy binary to APEX_INSTALL_DIR for linux
: ${USE_SUDO:="false"}

# Http request CLI
APEX_HTTP_REQUEST_CLI=curl

# GitHub Organization and repo name to download release
GITHUB_ORG=apexlang
GITHUB_REPO=cli

# apex CLI filename
APEX_CLI_FILENAME=apex

APEX_CLI_FILE="${APEX_INSTALL_DIR}/${APEX_CLI_FILENAME}"

getSystemInfo() {
    ARCH=$(uname -m)
    case $ARCH in
        armv7*) ARCH="arm";;
        aarch64) ARCH="arm64";;
        x86_64) ARCH="amd64";;
    esac

    OS=$(echo `uname`|tr '[:upper:]' '[:lower:]')

    # Most linux distro needs root permission to copy the file to /usr/local/bin
    if [[ "$OS" == "linux" || "$OS" == "darwin" ]] && [ "$APEX_INSTALL_DIR" == "/usr/local/bin" ]; then
        USE_SUDO="true"
    fi
}

verifySupported() {
    local supported=(darwin-amd64 darwin-arm64 linux-amd64 linux-arm)
    local current_osarch="${OS}-${ARCH}"

    for osarch in "${supported[@]}"; do
        if [ "$osarch" == "$current_osarch" ]; then
            echo "Your system is ${OS}_${ARCH}"
            return
        fi
    done

    echo "No prebuilt binary for ${current_osarch}"
    exit 1
}

runAsRoot() {
    local CMD="$*"

    if [ $EUID -ne 0 -a $USE_SUDO = "true" ]; then
        CMD="sudo $CMD"
    fi

    $CMD
}

checkHttpRequestCLI() {
    if type "curl" > /dev/null; then
        APEX_HTTP_REQUEST_CLI=curl
    elif type "wget" > /dev/null; then
        APEX_HTTP_REQUEST_CLI=wget
    else
        echo "Either curl or wget is required"
        exit 1
    fi
}

checkExistingapex() {
    if [ -f "$APEX_CLI_FILE" ]; then
        echo -e "\nApex CLI is detected:"
        $APEX_CLI_FILE version
        echo -e "Reinstalling Apex CLI - ${APEX_CLI_FILE}...\n"
    else
        echo -e "Installing Apex CLI...\n"
    fi
}

getLatestRelease() {
    local apexReleaseUrl="https://api.github.com/repos/${GITHUB_ORG}/${GITHUB_REPO}/releases"
    local latest_release=""

    if [ "$APEX_HTTP_REQUEST_CLI" == "curl" ]; then
        latest_release=$(curl -s $apexReleaseUrl | grep \"tag_name\" | grep -v rc | awk 'NR==1{print $2}' |  sed -n 's/\"\(.*\)\",/\1/p')
    else
        latest_release=$(wget -q --header="Accept: application/json" -O - $apexReleaseUrl | grep \"tag_name\" | grep -v rc | awk 'NR==1{print $2}' |  sed -n 's/\"\(.*\)\",/\1/p')
    fi

    ret_val=$latest_release
}

downloadFile() {
    LATEST_RELEASE_TAG=$1

    apex_CLI_ARTIFACT="${APEX_CLI_FILENAME}_${OS}_${ARCH}.tar.gz"
    DOWNLOAD_BASE="https://github.com/${GITHUB_ORG}/${GITHUB_REPO}/releases/download"
    DOWNLOAD_URL="${DOWNLOAD_BASE}/${LATEST_RELEASE_TAG}/${apex_CLI_ARTIFACT}"

    # Create the temp directory
    apex_TMP_ROOT=$(mktemp -dt apex-install-XXXXXX)
    ARTIFACT_TMP_FILE="$apex_TMP_ROOT/$apex_CLI_ARTIFACT"

    echo "Downloading $DOWNLOAD_URL ..."
    if [ "$APEX_HTTP_REQUEST_CLI" == "curl" ]; then
        curl -SsL "$DOWNLOAD_URL" -o "$ARTIFACT_TMP_FILE"
    else
        wget -q -O "$ARTIFACT_TMP_FILE" "$DOWNLOAD_URL"
    fi

    if [ ! -f "$ARTIFACT_TMP_FILE" ]; then
        echo "failed to download $DOWNLOAD_URL ..."
        exit 1
    fi
}

installFile() {
    tar xf "$ARTIFACT_TMP_FILE" -C "$apex_TMP_ROOT"
    local tmp_root_apex_cli="$apex_TMP_ROOT/${APEX_CLI_FILENAME}_${OS}_${ARCH}/$APEX_CLI_FILENAME"

    if [ ! -f "$tmp_root_apex_cli" ]; then
        echo "Failed to unpack Apex CLI executable."
        exit 1
    fi

    chmod o+x $tmp_root_apex_cli
    runAsRoot cp "$tmp_root_apex_cli" "$APEX_INSTALL_DIR"

    if [ -f "$APEX_CLI_FILE" ]; then
        echo "$APEX_CLI_FILENAME installed into $APEX_INSTALL_DIR successfully."

        $APEX_CLI_FILE version
    else 
        echo "Failed to install $APEX_CLI_FILENAME"
        exit 1
    fi
}

fail_trap() {
    result=$?
    if [ "$result" != "0" ]; then
        echo "Failed to install Apex CLI"
        echo "For support, go to https://apexlang.io"
    fi
    cleanup
    exit $result
}

cleanup() {
    if [[ -d "${apex_TMP_ROOT:-}" ]]; then
        rm -rf "$apex_TMP_ROOT"
    fi
}

installCompleted() {
    echo -e "\nApex CLI is installed successfully."
}

# -----------------------------------------------------------------------------
# main
# -----------------------------------------------------------------------------
trap "fail_trap" EXIT

getSystemInfo
verifySupported
checkExistingapex
checkHttpRequestCLI


if [ -z "$1" ]; then
    echo "Getting the latest Apex CLI..."
    getLatestRelease
else
    ret_val=v$1
fi

echo "Installing $ret_val Apex CLI..."

downloadFile $ret_val
installFile
cleanup

installCompleted
