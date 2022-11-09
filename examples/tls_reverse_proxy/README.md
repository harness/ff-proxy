# TLS Reverse Proxy
This example will spin up a relay proxy sitting behind nginx with TLS enabled acting as a reverse proxy to handle HTTPS connections from external sdks.

Note this is only a quickstart example config and should not be used for production purposes.

# Add certs
Place your required cert and private key in the /certs folder named `cert.crt` and `cert.key`
Alternatively you can generate some self signed certs with a command like this (or any equivalent)

```openssl req -x509 -sha256 -nodes -newkey rsa:2048 -days 365 -keyout reverseproxy/certs/cert.key -out reverseproxy/certs/cert.crt ```

# Running
1. Add your relay proxy details to the .env file.
2. Add your certs to the /certs folder as described ^
3. `docker-compose --env-file .env up  proxy  nginx`
4. Connect sdks to `https://localhost:8000` if local or whatever url your nginx is listening on otherwise

