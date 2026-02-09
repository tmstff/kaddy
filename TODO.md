# ToDo for this project

## v0

- [ ] improve e2e tests
- [ ] fix PVC Problem
- [ ] improve unit tests
- [ ] get github pipeline for e2e tests running
  - setup registry.redhat.io authentication in a github action
    1. https://www.ecosia.org/search?q=How+to+setup+registry.redhat.io+authentication+in+a+github+action+for+operator-sdk%3F
    2. https://gist.github.com/nickboldt/a884449ada7a6cdcda85e58c984b5eea
- [ ] create cluster role & binding for PVC access
- [ ] create route
- [ ] improve retry & backoff
- [ ] make TLS configurable (internal vs external)
- [ ] make response configurable (e.g. provide content of PVC)
- [ ] Fine Tuning
	1. rename Kaddy to Caddy?
	2. make resource requirements configurable?

## v1

- [ ] provide proxy functionality