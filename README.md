[![Build Status](https://travis-ci.com/skycoin/skywire.svg?branch=master)](https://travis-ci.com/skycoin/skywire)

# Skywire

- [Skywire](#skywire)
    - [Build](#build)
    - [Configure Skywire](#configure-skywire)
        - [Expose hypervisorUI](#expose-hypervisorui)
        - [Add remote hypervisor](#add-remote-hypervisor)
    - [Run `skywire-visor`](#run-skywire-visor)
        - [Using the Skywire VPN](#using-the-skywire-vpn)
    - [Creating a GitHub release](#creating-a-github-release)
        - [How to create a GitHub release](#how-to-create-a-github-release)

## Build


### Add remote hypervisor

Every visor can be controlled by one or more hypervisors. To allow a hypervisor to access a visor, the PubKey of the
hypervisor needs to be specified in the configuration file. You can add a remote hypervisor to the config with:

```bash
$ skywire-cli visor update-config --hypervisor-pks <public-key>
```

Or from docker image:

```bash
$ docker run --rm -v <YOUR_CONFIG_DIR>:/opt/skywire \
  skycoin/skywire:latest skywire-cli update-config hypervisor-pks <public-key>

```

## Run `skywire-visor`

`skywire-visor` hosts apps and is an applications gateway to the Skywire network.

`skywire-visor` requires a valid configuration to be provided. If you want to run a VPN client locally, run the visor
as `sudo`.

```bash
$ sudo skywire-visor -c skywire-config.json
```

Or from docker image:

```bash
docker run --rm -p 8000:8000 -v <YOUR_CONFIG_DIR>:/opt/skywire --name=skywire skycoin/skywire:latest skywire-visor
```

`skywire-visor` can be run on Windows. The setup requires additional setup steps that are specified
in [the docs](docs/windows-setup.md).

### Using the Skywire VPN

If you are interested in running the Skywire VPN as either a client or a server, please refer to the following guides:

- [Setup the Skywire VPN](https://github.com/skycoin/skywire/wiki/Setting-up-Skywire-VPN)
- [Setup the Skywire VPN server](https://github.com/skycoin/skywire/wiki/Setting-up-Skywire-VPN-server)

## Creating a GitHub release

To maintain actual `skywire-visor` state on users' Skywire nodes we have a mechanism for updating `skywire-visor`
binaries. Binaries for each version are uploaded to [GitHub releases](https://github.com/skycoin/skywire/releases/). We
use [goreleaser](https://goreleaser.com) for creating them.

### How to create a GitHub release

1. Make sure that `git` and [goreleaser](https://goreleaser.com/install) are installed.
2. Checkout to a commit you would like to create a release against.
3. Run `go mod vendor` and `go mod tidy`.
4. Make sure that `git status` is in clean state. Commit all vendor changes and source code changes.
5. Uncomment `draft: true` in `.goreleaser.yml` if this is a test release.
6. Create a `git` tag with desired release version and release name: `git tag -a 0.1.0 -m "First release"`,
   where `0.1.0` is release version and `First release` is release name.
5. Push the created tag to the repository: `git push origin 0.1.0`, where `0.1.0` is release version.
6. [Issue a personal GitHub access token.](https://github.com/settings/tokens)
7. Run `GITHUB_TOKEN=your_token make github-release`
8. [Check the created GitHub release.](https://github.com/skycoin/skywire/releases/)
