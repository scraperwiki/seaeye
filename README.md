# Seaeye

## Background

If working closely with Github, starts with reading
[Github: Building a CI server](https://developer.github.com/guides/building-a-ci-server/)
as an adequate primer.


## Overview

A simple continuous integration server using
[Hookbot](https://github.com/scraperwiki/hookbot) to subscribe to branch changes
on Github hosted repositories.

Assumptions:

- The CI server has access to write files (git clones) to its current working
directory on start time.
- Repository branch changes are published to a topic on a Hookbot server
  instance the CI server can subscribe to.
- Repositories are cloned using the
  [git-prep-directory](https://github.com/scraperwiki/hanoverd/blob/master/cmd/git-prep-directory/main.go)
  command instead of `git clone`.
- `.seaeye.yml` configuration file in the root of the repository


## Workflow

TBD.

When using Docker-out-of-Docker (DooD) and volume mounts are required but the
paths inside and outside can't be made equal, the volume path inside must be a
prefix to the outside path _and_ the special environment variable
`SEAYEYE_WORKSPACE` must be set. E.g.:

     -v /seaeye/workspace=/seaeye/workspace # OK
     :
     -e SEAEYE_WORKSPACE=/seaeye/workspace -v /data/seaeye/workspace=/seaeye/workspace # OK
     :
     -e SEAEYE_WORKSPACE=/seaeye/workspace -v /data/seaeye/workspace=/seaeye # NOT OK


## Setup

Any interaction with Github initiated by Seaeye is authenticated and authorized
against a Github user. This allows pulling from repositories and updating commit
statuses. Don't add the machine user as collaborator but as a member to the new
team.

1. [Generate a new SSH key](https://help.github.com/articles/generating-an-ssh-key/)
2. [Create a new Github Team](https://help.github.com/articles/creating-a-team/)
   (e.g. `bots`)
3. [Create a new Github _Machine user_](https://help.github.com/articles/signing-up-for-a-new-github-account/)
   (e.g. `seaeye`)
4. [Add the new organization member to the new team](https://help.github.com/articles/adding-organization-members-to-a-team/)

**Note:** Adding a machine user as a collaborator always grants read/write
access while adding a machine user to a team grants the permissions of the team.

**Note:** Automating the creation of accounts is prohibited by Github's ToS:

> Accounts registered by "bots" or other automated methods are not permitted.

**Note:** Most private Github repositories have Git submodules linked to other
private Github repositories, so using Github's _Deploy keys_ would require a
more complex setup (configuring Seaeye's access rights on the server side
instead of on Github administration side, which will require e.g. CloudFormation
changes) or reusing the same key. It also leaves the task of pushing commit
statuses open.


## Resources

- [Github: Building a CI server](https://developer.github.com/guides/building-a-ci-server/)
- [Github: Managing deploy keys](https://developer.github.com/guides/managing-deploy-keys/)
