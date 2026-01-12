#! /usr/bin/env bash
# Generates a release from this branch with the given version. Also tags this to the repo
# release <branch> <version>
#    <branch> is the branch we need to use to do the build
#    <version> is the tag we will add to the branch once we complete the build


set -xe

# Validates the given argument both in syntax and also ensures clean repo for build
function validate_arguments {

	# Vaidate the number of arguments and the syntax
    if [ $# -ne 1 ]; then
	    echo "Error: Invalid number of arguments"
		echo "Usage: basename($0) <branch> <version>"
		echo "   branch - branch to checkout and build"
		echo "   version - version to build and tag the branch"
		exit 1
	fi

	# Before attempting to build, move to the appropriate directory
    script_source="$( cd "$(dirname "${BASH_SOURCE[0]}" )" && pwd)"
    cd $script_source

	if ! echo $version | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' > /dev/null; then
		echo "Error: Version not in the proper format"
		echo "Versino: should be of the format M.m.p"
		echo "where M - major, m - minor and p - patch are all integers"
		echo "got version as $2"
		exit 1
	fi

	# Checkout the requestd branch and make sure the local repo is clean
	if ! git checkout $1 > /dev/null; then
		echo "checkout of the requested branch $1 failed. See the above error message"
		exit 1
	fi

	# Make sure that this is a clean local repo.
	# For now comment this out as we are in development of this script
	out=`git status --short`
	if [[ "$out" != "" ]] ; then
		echo "your git branch $1 in the local repo is not clean. Cannot build"
		exit
	fi
	
	# Return the base directory to the other functions
	echo `dirname $script_source`
}

# Given the version, os and arch sets up the artifact for this combination
function build_artifact {
	# Build this combination first
	if ! CGO_ENABLED="0" GOOS=$os GOARCH=$arch go build ./terraform-provider; then
		echo "Unable to build for $od and $arch"
		exit
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

version=`cat release_version.txt`

base_dir=`validate_arguments $*`

echo $version
if [ "$version" == "" ]; then
	echo "Version is not set"
	exit 2
fi

# Loop over the build variants and set up each of these in the artifact
for os in "${!build_variants[@]}"; do
	declare -a arch_list=(${build_variants[$os]})
	for arch in "${arch_list[@]}"; do
		echo "OS: ${os}, arch: ${arch}"
	    build_artifact $base_dir $version $os $arch
	done
done

