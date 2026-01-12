#! /usr/bin/env python3

'''
   Generates all the artifacts that are needed to host a binary in the TF registry
'''

import os
import argparse
import hashlib
import subprocess
import json
from zipfile import ZipFile, ZIP_DEFLATED

# pylint: disable=too-many-function-args:w

ARTIFACT_DIR = "fm_terraform_provider/terraform-provider/artifacts"
GPG_DIR = "fm_terraform_provider/terraform-provider/build/gpg_keys"
BUILD_DIR = "fm_terraform_provider/terraform-provider/build"

def get_file_from_components(os_type, arch_type, version):
    '''form the file name prefix for all components from the above fields'''
    return f'terraform-provider-gigamon_{version}_{os_type}_{arch_type}'

def store_zip(file_name, os_type, arch_type, version, artifact_dir):
    '''Zip the given binary file and store it with the correct name'''
    zip_name = get_file_from_components(os_type, arch_type, version) + ".zip"
    artifact_file = os.path.join(artifact_dir, zip_name)
    with ZipFile(artifact_file, 'w', compression=ZIP_DEFLATED, compresslevel=9) as zipf:
        zipf.write(file_name, arcname=zip_name)
    return zip_name

def get_sha256(os_type, arch_type, version, artifact_dir):
    '''Get the sha256 checksum and create the corresponding file'''
    zip_name = get_file_from_components(os_type, arch_type, version) + ".zip"
    sha256_name = get_file_from_components(os_type, arch_type, version) + ".sha256"

    # Get the SHA256
    hash_func = hashlib.sha256()
    with open(os.path.join(artifact_dir, zip_name), 'rb') as f:
        data = f.read(4096)
        while data:
            hash_func.update(data)
            data = f.read(4096)
    hash_out = hash_func.hexdigest()
    with open(os.path.join(artifact_dir, sha256_name), "w", encoding="utf-8") as f:
        f.write(f'{hash_out}  {zip_name}\n')
    return hash_out, sha256_name

def get_signed_hash(os_type, arch_type, version, artifact_dir, gpg_dir):
    '''Get the GPG signature of the hash of this file'''
    sha256_name = get_file_from_components(os_type, arch_type, version) + ".sha256"
    sig_name = get_file_from_components(os_type, arch_type, version) + ".sig"
    cmd = [
        "gpg",
        "--homedir",
        gpg_dir,
        "--batch",
        "--output",
        os.path.join(artifact_dir, sig_name),
        "--yes",
        "--detach-sig",
        os.path.join(artifact_dir, sha256_name),
    ]
    proc = subprocess.run(
        cmd,
        encoding="utf-8",
        capture_output=True,
        check=False,
        timeout=120,
    )
    if proc.returncode != 0:
        raise ValueError(
            'gpg signature command failed\n'
            f'Command: {cmd}, returncode: {proc.returncode}\n'
            f'Stdout: {proc.stdout}\n'
            f'Stderr: {proc.stderr}\n'
        )
    return sig_name

def get_key_details(build_dir):
    '''Returns the public key details, that is saved in the build directory'''
    with open(os.path.join(build_dir, "gpg_keyid.txt"), "r", encoding="utf-8") as fhdl:
        gpg_key = fhdl.read().strip()
    with open(os.path.join(build_dir, "gpg_key_armor.txt"), "r", encoding="utf-8") as fhdl:
        key_armor = fhdl.read().strip()

    return gpg_key, key_armor

def update_versions(os_type, arch_type, version, artifact_dir):
    '''Update this os/arch/version in the versinos API response data'''
    version_file = os.path.join(artifact_dir, "version.json")
    try:
        with open(version_file, "r", encoding="utf-8") as fhdl:
            version_resp = json.loads(fhdl.read())
    except FileNotFoundError as exc:
        version_resp = {"versions": []}

    # Check if this provider version is already there
    for prov_ver in version_resp["versions"]:
        if prov_ver["version"] == version: # We already have an entry for this version
            for platform in prov_ver["platforms"]:
                if platform["os"] == os_type and platform["arch"] == arch_type:
                    # Already present nothing to do
                    break
            else:
                prov_ver["platforms"].append({"os": os_type, "arch": arch_type})
            break
    else:
        version_resp["versions"].append({
            "version": version,
            "protocols": ["6.0"],
            "platforms": [
                {"os": os_type, "arch": arch_type},
            ],
        })

    with open(version_file, "w", encoding="utf-8") as fhdl:
        fhdl.write(json.dumps(version_resp, indent=4))

def main():
    '''Get the arguments and sign the binary and prepare the metadata'''
    parser = argparse.ArgumentParser()
    parser.add_argument(
        '--binary',
        help='The name of the binary, that we want to host for registry',
        required=True,
    )

    parser.add_argument(
        '--os',
        help='The os for which this binary is generated',
        required=True,
    )

    parser.add_argument(
        '--arch',
        help='The architecture (processor family) for which this binary is generated',
        required=True,
    )

    parser.add_argument(
        '--version',
        help='The version of the provider',
        required=True,
    )

    parser.add_argument(
        '--base_dir',
        help='The base directory where the repo is present',
        required=True,
    )

    args = parser.parse_args()

    # Now prepare the binary, i.e. create a zip out of it and host it in the artifact directory
    artifact_dir = os.path.join(args.base_dir, ARTIFACT_DIR)
    gpg_dir = os.path.join(args.base_dir, GPG_DIR)
    build_dir = os.path.join(args.base_dir, BUILD_DIR)

    zip_file_name = store_zip(args.binary, args.os, args.arch, args.version, artifact_dir)
    hash_out, hash_file = get_sha256(args.os, args.arch, args.version, artifact_dir)
    sig_file = get_signed_hash(args.os, args.arch, args.version, artifact_dir, gpg_dir)

    # prepare the overall metadata to send to TF on download request
    key_id, key_armor = get_key_details(build_dir)
    meta_data = {
        "protocols": ["6.0"],
        "os": args.os,
        "arch": args.arch,
        "filename": zip_file_name,
        "download_url": (
            f'https://tf-proj.gigamon.com/terraform-provider-gigamon/2.0.0/{zip_file_name}'
        ),
        "shasums_url": (
            f'https://tf-proj.gigamon.com/terraform-provider-gigamon/2.0.0/{hash_file}'
        ),
        "shasums_signature_url": (
            f'https://tf-proj.gigamon.com/terraform-provider-gigamon/2.0.0/{sig_file}'
        ),
        "shasum": hash_out,
        "signing_keys": {
            "gpg_public_keys": [
                {
                    "key_id": key_id,
                    "ascii_armor": key_armor,
                    "trust_signature": "",
                    "source": "gigamon",
                    "source_url": "https://tf-proj.gigamon.com/security.html",
                },
            ],
        },
    }
    meta_file = get_file_from_components(args.os, args.arch, args.version) + ".meta"
    with open(os.path.join(artifact_dir, meta_file), "w", encoding="utf-8") as fhdl:
        fhdl.write(json.dumps(meta_data, indent=4, sort_keys=True))

    # Update the versions file
    update_versions(args.os, args.arch, args.version, artifact_dir)

main()
