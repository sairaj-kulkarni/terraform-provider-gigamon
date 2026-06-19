#! /usr/bin/env bash


#  Copyright (c) 2017-2026 Gigamon, Inc. All rights reserved.
#
#  Author: Gigamon Terraform Team (gigamon-terraform-team@gigamon.com)
#
#  This program is free software: you can redistribute it and/or modify
#  it under the terms of the GNU General Public License as published by
#  the Free Software Foundation, version 3 of the License.
#
#  This program is distributed in the hope that it will be useful,
#  but WITHOUT ANY WARRANTY; without even the implied warranty of
#  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
#  GNU General Public License for more details.
#
#  You should have received a copy of the GNU General Public License
#  along with this program. If not, see <https://www.gnu.org/licenses/>



#
# A script, that sets up the gpg keys that we will use for signging the provider binary.
#
set -e # Fail on any command erroring out
set -o pipefail # Set the error status of the pipe as the last command that failed

# Make sure that the directory for the GPG keys exist
mkdir -p gpg_keys

# Ensure that this has the right permissions
chmod 0700 gpg_keys

# Generate our GPG keys
gpg --homedir gpg_keys --batch --gen-key genkey.txt > gpg_errors 2>&1

# Store the key-id and public key in armor format
keyid=`gpg --homedir gpg_keys --list-secret-keys --keyid-format=long | grep sec | cut -f 2 -d '/' | cut -f 1 -d ' '`
key_armor=`gpg --homedir gpg_keys --armor --export $keyid`

# Store these in files
echo "$keyid" > gpg_keyid.txt
echo "$key_armor" > gpg_key_armor.txt
