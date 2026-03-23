#! /usr/bin/env bash

# Generates a release from this branch with the given version. Also tags this to the repo
# release [branch ]  <enable_code_coverage>
#    [branch] is the branch we need to use to do the build
#    <enable_code_coverage> optional parameter that enables code coverage in the binary
#                 that is generated. Accepts true/false as the argument. Defaults to false


set -xeuo pipefail

function update_version {
    version=`cat release_version.txt | cut -f 3 -d '.'`
    ((version = version + 1))
    new_version=`cat release_version.txt | cut -f '1 2' -d '.'` 
    new_version="${new_version}.${version}"
    echo creating version - $new_version
    echo ${new_version} > release_version.txt
    git add release_version.txt
    git commit --message "Version updated"
    git push
}

# Validates the given argument both in syntax and also ensures clean repo for build
function validate_arguments {

    # Vaidate the number of arguments and the syntax
    if [[ $# -eq 0 ]] || [[ $# -gt 2 ]]; then
        set +x # Stop the echoing so that the output looks clean
        echo "Error: Invalid number of arguments"
        echo "Usage: $0 [branch] <enable_code_coverage>"
        echo "   branch - branch to checkout and build. for e.g. main. Mandatory parameter"
        echo "   enable_code_coverage - optional parameter that enables code coverage"
        echo "       Can be set to true/false. Defaults to false, i.e. code coverage disabled" 
        exit 1
    fi

    if [[ $# -eq 2 ]]; then
        if [[ $2 != "true" ]] && [[ $2 != "false" ]]; then
            set +x # Stop echoing to make the outut look clean
            echo "Invalid value: $2 for enable_code_coverage"
            echo "Usage: $0 [branch] <enable_code_coverage>"
            echo "   branch - branch to checkout and build. for e.g. main. Mandatory parameter"
            echo "   enable_code_coverage - optional parameter that enables code coverage"
            echo "    Can be set to true/false. Defaults to false, i.e. code coverage disabled" 
            exit 1
        fi
    fi

    # Before attempting to build, move to the appropriate directory
    script_source="$( cd "$(dirname "${BASH_SOURCE[0]}" )" && pwd)"
    cd $script_source


    # Checkout the requestd branch and make sure the local repo is clean
    if ! git checkout $1 > /dev/null 2>&1 ; then
        echo "checkout of the requested branch $1 failed. See the above error message"
        exit 1
    fi

    if ! git pull > /dev/null 2>&1 ; then
        echo "pull of the requested branch $1 failed. See the above error message"
        exit 1
    fi

    # Make sure that this is a clean local repo.
    out=`git status --short`
    if [[ "$out" != "" ]] ; then
        echo "your git branch $1 in the local repo is not clean. Cannot build"
        exit 1
    fi

    # Bump up the version by 1, and commit that back to the repo

    update_version

    version=`cat release_version.txt`
    if ! echo $version | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' > /dev/null 2>&1; then
        echo "Error: Version not in the proper format"
        echo "Versino: should be of the format M.m.p"
        echo "where M - major, m - minor and p - patch are all integers"
        echo "got version as $2"
        exit 1
    fi
}

# Given the version, os and arch sets up the artifact for this combination
function build_artifact {
    # Build this combination first
    if ! CGO_ENABLED="0" GOOS=$3 GOARCH=$4 go build $5 -ldflags "-X 'main.version=v$2'" ./terraform-provider; then
        echo "Unable to build for $3 and $4"
        exit 1
    fi

    # Form the artifact for this version/os/arch
    terraform-provider/build/build.py --binary terraform-provider-gigamon --os $3 --arch $4 --version $2 --base_dir $1
}


# This OS and architectures that we are going to be supporting
declare -A build_variants
build_variants["linux"]="amd64 arm64"
build_variants["darwin"]="amd64 arm64"
build_variants["windows"]="amd64"

# Validate the arguments, and also change our working directory to the root of the git repo
# base_name will contain the directory where the repo is present

validate_arguments $*
if [[ $# -eq 2 ]] && [[ $2 -eq "true" ]]; then
    code_coverage="-cover"
else
    code_coverage=""
fi

script_source="$( cd "$(dirname "${BASH_SOURCE[0]}" )" && pwd)"
base_dir=`dirname $script_source`

version=`cat release_version.txt`

# Loop over the build variants and set up each of these in the artifact
for os in "${!build_variants[@]}"; do
    declare -a arch_list=(${build_variants[$os]})
    for arch in "${arch_list[@]}"; do
        echo "OS: ${os}, arch: ${arch}"
        build_artifact $base_dir $version $os $arch "$code_coverage"
    done
done

# Tag the repo with this version
git tag --annotate v$version --message "Release Version $version"
git push origin v$version
