/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
 @Version : 1.0
 @Author  : steven.wang
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2022/2022/14 14/32/15
 @Desc    :
*/

package cipher

import (
	"testing"
)

var (
	kb1 = `u-00000001:o-00000001:test-1:1750777732`
	kb2 = ``
	kb3 = `#!/bin/bash

DRY_RUN=false

log() {
  local level="${1:-INFO}"
  local message="$2"
  local timestamp=$(date "+%Y-%m-%d %H:%M:%S")
  
  # set color based on log level
  local color=""
  local reset="\033[0m"
  
  case "$level" in
    INFO)  color="\033[0;32m" ;; # green
    WARN)  color="\033[0;33m" ;; # yellow
    ERROR) color="\033[0;31m" ;; # red
    *)     color="\033[0m"    ;; # default
  esac
  
  # output to console (with color) and log file (without color)
  if [ "$DRY_RUN" = false ]; then
    echo -e "${color}[${timestamp}] [${level}] ${message}${reset}" >> {{.LOG_PATH}}
  else
    echo -e "${message}"
  fi
}

init() {
  mkdir -p {{.MOUNT_POINT}} || true
  mkdir -p {{.JUICEFS_CACHE_DIR}} || true

  CURL_PATH=$(which curl 2>/dev/null || echo "")
  if [ -z "$CURL_PATH" ]; then
    apt update
    apt install -y curl
  else
    log INFO "curl already installed at $CURL_PATH"
  fi

  FUSE_PATH=$(which fusermount 2>/dev/null || echo "")
  if [ -z "$FUSE_PATH" ]; then
    apt update
    apt install -y fuse
  else
    log INFO "fuse already installed at $FUSE_PATH"
  fi

  # check gcloud cli
  GCLOUD_PATH=$(which gcloud 2>/dev/null || echo "")
  if [ -z "$GCLOUD_PATH" ]; then
    log INFO "gcloud cli is not installed, installing it..."
    # add google cloud sdk source
    echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | tee -a /etc/apt/sources.list.d/google-cloud-sdk.list

    # install apt-transport-https ca-certificates gnupg
    apt-get install -y apt-transport-https ca-certificates gnupg
    curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key --keyring /usr/share/keyrings/cloud.google.gpg add -

    # update and install cloud sdk
    apt-get update
    apt-get install -y google-cloud-sdk
  else
    log INFO "gcloud cli already installed at $GCLOUD_PATH"
  fi

  # create gcloud config dir
  if [ ! -d ~/.config/gcloud ]; then
    mkdir -p ~/.config/gcloud
  fi

  if [ ! -f ~/.config/gcloud/application_default_credentials.json ]; then
    # create application_default_credentials.json
    cat <<EOF > ~/.config/gcloud/application_default_credentials.json
{{.GCP_APPLICATION_DEFAULT_CREDENTIALS}}
EOF
  fi
}

mount() {
  init
  # install JuiceFS client
  if [ ! -f "{{.JUICEFS_PATH}}" ]; then
    log INFO "JuiceFS client not found, downloading..."
    curl -L {{.JUICEFS_CONSOLE_HOST}}/onpremise/juicefs -o {{.JUICEFS_PATH}} && chmod +x {{.JUICEFS_PATH}}
  else
    # get current installed JuiceFS version
    CURRENT_VERSION=$({{.JUICEFS_PATH}} version 2>/dev/null | awk '{print $3}')
    # compare version
    if [ "$CURRENT_VERSION" != "{{.JUICEFS_VERSION}}" ]; then
      log INFO "JuiceFS version mismatch (current: $CURRENT_VERSION, required: {{.JUICEFS_VERSION}}), downloading new version..."
      curl -L {{.JUICEFS_CONSOLE_HOST}}/onpremise/juicefs -o {{.JUICEFS_PATH}} && chmod +x {{.JUICEFS_PATH}}
      {{.JUICEFS_PATH}} version -u
    fi
  fi

  {{.JUICEFS_PATH}} mount --cache-dir {{.JUICEFS_CACHE_DIR}} --token {{.JUICEFS_TOKEN}} {{.JUICEFS_NAME}} {{.MOUNT_POINT}} {{.MOUNT_OPTIONS}} || exit 1

}

umount() {
  {{.JUICEFS_PATH}} umount {{.MOUNT_POINT}}
  rm -rf {{.MOUNT_POINT}}
  if [ -f "{{.STORAGE_PATH}}" ]; then
    rm -f "{{.STORAGE_PATH}}"
    log INFO "storage path removed: {{.STORAGE_PATH}}"
  fi
  log INFO "{{.JUICEFS_NAME}} umount done"
}

dry_run() {
  echo "dry run"
  DRY_RUN=true
  init
  mount
  umount
  DRY_RUN=false
}

main(){
  case $1 in
    mount) mount ;;
    umount) umount ;;
    dry-run) dry_run ;;
    *) log ERROR "Usage: $0 {mount|umount|dry-run}" ;;
  esac
}

main "$@"`
)

func TestEncrypt(t *testing.T) {
	cases := []struct {
		Name string
		text []byte
	}{
		{"a", []byte(kb1)},
		{"b", []byte(kb2)},
		{"c", []byte(kb3)},
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if ans, err := EncryptCompact(c.text, "uOvKLmVfztaXGpNYd4Z0I1SiT7MweJhl"); err != nil {
				t.Fatalf("encrypt text %s failed: %+v",
					c.text, err)
			} else {
				t.Logf("%s encrypt text is { %s }", c.Name, ans)
			}
		})
	}
}

func TestDecrypt(t *testing.T) {
	cases := []struct {
		Name string
		text string
	}{
		{"a", ``},
		{"b", ""},
		{"c", ""},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if ans, err := DecryptCompact(c.text, "uOvKLmVfztaXGpNYd4Z0I1SiT7MweJhl"); err != nil {
				t.Fatalf("decrypt text %s failed: %+v", c.text, err)
			} else {
				t.Logf("decrypt is %s", ans)
			}
		})
	}
}
