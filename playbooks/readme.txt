tf-proj.gigamon.com is the server for terraform project and is used for the following

To meet the various use cases, please ensure the following before you start the ansible

1. Create a user called jana and ensure that we are able to do passwordless login to this
   user. This is the user home under which we will host the provider repository and also 
   all the builds will be done from here

2. Create a user called gocode, and this will be used for setting up the code coverage

 Both these users should be setup for passwordless login and also should have sudo permission
 and both should be able to do git operations on our gitlab TF repo. Ensure that their git
 permissions are setup appropriately

 After this run ansible playbook from here as
 ansible-playbook --inventory inventory.yml --ask-become-pass --become site.yml

1. It is hosting the TF provider code for the internal test team or other groups
   use.

   This is build using the release.sh in the base directory, which builds all the binaries
   (all OS and Arch) and hosts them in the artifact directory along with thie SHA/other
   things that are needed

   For doing this, login as jana, and go to the fm_terraform_provider and run ./release.sh
   and if needed set code coverage enable as true (in case we want the build to have code
   coverage enabled)

2. It is hosting the code-coverage temp. location where when a test is being run, the code
   coverage intermediate files are all uploaded (into /code-coverage/files/{provider-version}.

3. It is also used by the script to combine and analyze all these code coverage counters. this
   is done by the user gocode and the repo is checked out in /code-coverage/src

