#!/bin/bash
# source - https://github.com/fanout/docker-pushpin/blob/master/docker-entrypoint.sh
set -e

# Configure Pushpin
if [ -w /usr/lib/pushpin/internal.conf ]; then
	sed -i \
		-e 's/zurl_out_specs=.*/zurl_out_specs=ipc:\/\/\{rundir\}\/pushpin-zurl-in/' \
		-e 's/zurl_out_stream_specs=.*/zurl_out_stream_specs=ipc:\/\/\{rundir\}\/pushpin-zurl-in-stream/' \
		-e 's/zurl_in_specs=.*/zurl_in_specs=ipc:\/\/\{rundir\}\/pushpin-zurl-out/' \
		/usr/lib/pushpin/internal.conf
else
	echo "docker-entrypoint.sh: unable to write to /usr/lib/pushpin/internal.conf, readonly"
fi

if [ -w /etc/pushpin/pushpin.conf ]; then
	sed -i \
		-e 's/services=.*/services=condure,zurl,pushpin-proxy,pushpin-handler/' \
		-e 's/push_in_spec=.*/push_in_spec=tcp:\/\/\*:5560/' \
		-e 's/push_in_http_addr=.*/push_in_http_addr=0.0.0.0/' \
		-e 's/push_in_sub_specs=.*/push_in_sub_spec=tcp:\/\/\*:5562/' \
		-e 's/command_spec=.*/command_spec=tcp:\/\/\*:5563/' \
		/etc/pushpin/pushpin.conf
else
	echo "docker-entrypoint.sh: unable to write to /etc/pushpin/pushpin.conf, readonly"
fi

# Set routes with ${target} for backwards-compatibility.
if [ -v target ]; then
	echo "* ${target},over_http" > /etc/pushpin/routes
fi

# Update pushpin.conf file to use $PORT for http_port
if [ -w /etc/pushpin/pushpin.conf ]; then
  PROTOCOL="http_port"
  PUSHPIN_PORT=7000
  if [ -n "${PORT}" ]; then
      PUSHPIN_PORT=${PORT}
  fi

  if [ "${TLS_ENABLED}" = true ] ; then
    echo "https configured"
    PROTOCOL="https_ports"

    # write ca cert to pushpin certs directory if exists
    if [ -n "${TLS_CERT}" ]; then
      echo "copying tls cert from ${TLS_CERT} to etc/pushpin/runner/certs/default_${PUSHPIN_PORT}.crt"
      cp ${TLS_CERT} etc/pushpin/runner/certs/default_${PUSHPIN_PORT}.crt
    fi

    # write ca key to pushpin certs directory if exists
    if [ -n "${TLS_KEY}" ]; then
      echo "copying tls key from ${TLS_CERT} to etc/pushpin/runner/certs/default_${PUSHPIN_PORT}.key"
      cp ${TLS_KEY} etc/pushpin/runner/certs/default_${PUSHPIN_PORT}.key
    fi
  fi

  # set port and protocol for pushpin to listen on e.g. listen for https connections on port 6000
  echo "Listening for requests on port ${PUSHPIN_PORT}"
  sed -i \
  -e "s/http_port=7000/${PROTOCOL}=${PUSHPIN_PORT}/" \
  /etc/pushpin/pushpin.conf
  export PORT=
else
	echo "docker-entrypoint.sh: unable to write to /etc/pushpin/pushpin.conf, readonly"
fi

# Update routes file to forward traffic using ssl if tls_enabled is true
if [ -w /etc/pushpin/routes ]; then
  if [ "${TLS_ENABLED}" = true ] ; then
    sed -i \
      -e "s/localhost:8000/localhost:8000,ssl,insecure/" \
      /etc/pushpin/routes
  fi
else
	echo "docker-entrypoint.sh: unable to write to /etc/pushpin/routes, readonly"
fi

exec "$@"