#! /usr/bin/env python3

'''
   Generates all the artifacts that are needed to host a binary in the TF registry
'''

import os
import argparse
import hashlib
from zipfile import ZipFile, ZIP_DEFLATED

def get_file_from_components(os_type, arch_type, version):
    '''form the file name prefix for all components from the above fields'''
    return f'terraform-provider-gigamon_{version}_{os_type}_{arch_type}'

def store_zip(file_name, os_type, arch_type, version, artifact_dir):
    '''Zip the given binary file and store it with the correct name'''
    zip_name = get_file_from_components(os_type, arch_type, version) + ".zip"
    artifact_file = os.path.join(artifact_dir, zip_name)
    with ZipFile(artifact_file, 'w', compression=ZIP_DEFLATED, compresslevel=9) as zipf:
        zipf.write(file_name)
    return artifact_file

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
    return hash_out


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

    parser.add_argument(
        '--artifact_dir',
        help='The directory where the artifacts are stored, in relation to the repo base',
        default="fm_terraform_provider/terraform-provider/artifacts",
    )


    args = parser.parse_args()

    # Now prepare the binary, i.e. create a zip out of it and host it in the artifact directory
    artifact_dir = os.path.join(args.base_dir, args.artifact_dir)
    zip_file_name = store_zip(args.binary, args.os, args.arch, args.version, artifact_dir)
    hash_out = get_sha256(args.os, args.arch, args.version, artifact_dir)

main()
