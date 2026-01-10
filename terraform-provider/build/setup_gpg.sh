#! /usr/bin/env bash
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
