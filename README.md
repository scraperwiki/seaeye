# CI

A simple continuous integration server using
[hookbot](https://github.com/scraperwiki/hookbot) to subscribe to branch changes
on Github hosted repositories.

Assumptions:

- The CI server has access to write files (git clones) to its current working
directory on start time.
- Repository branch changes are published to a topic on a Hookbot server
  instance the CI server can subscribe to.
- Repositories are cloned using the
  [git-prep-directory](https://github.com/scraperwiki/hanoverd/blob/master/cmd/git-prep-directory/main.go)
  command instead of `git clone`.
- Either `make ci` target or `.seaeye.yml` configuration file

## Workflow

![Workflow](http://www.websequencediagrams.com/cgi-bin/cdraw?lz=dGl0bGUgQ0kgd29ya2Zsb3cKCkNJLT5Ib29rYm90OiBTdWIKR2l0aHViLQALC1B1YiAoTmV3IHB1c2gpCgAnBy0tPkNJOiBQdWxsABIMQ0ktLT4APgYAMwdQZW5kaW5nOiBDbG9uaW5nAB8FADcFQ2xvbmUAFhxCdWlsZGluZyAmIFRlc3QAMA0AFwUAEgcAYxNTdWNjZXNzL0ZhaWx1cmUpCgo&s=napkin)
