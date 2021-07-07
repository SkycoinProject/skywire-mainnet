#!/bin/sh

test -d /opt/skywire || {
  echo "no docker volume mounted, exiting..."
  exit 1
}

if [ "$#" -ne 1 ]; then
  test -f /opt/skywire/skywire-config.json || {
    echo "no config found, generating one...." &&
      /bin/skywire-cli visor gen-config -o /opt/skywire/skywire-config.json -r --is-hypervisor &&
      sed -i 's/localhost//g' /opt/skywire/skywire-config.json &&
      echo "config generated." &&
      exit 0
  }
fi

cmd="$(echo "$1" | tr -d '[:space:]')"
shift 1

echo "$@"

case "$cmd" in
skywire-visor)
  ./"$cmd" -c /opt/skywire/skywire-config.json "$@"
  ;;
skywire-cli)
  /bin/skywire-cli "$@"
  ;;
skychat | skysocks | skysocks-client)
  /apps/"$cmd" "$@"
  ;;
esac
