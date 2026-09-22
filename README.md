# Split-Proxy

Split-Proxy is an HTTP/SOCKS5 proxy server that allows you to configure and control user traffic routing based on customizable rules.

The project can be useful for:

* **Corporate infrastructure** — routing traffic between different networks, locations, and gateways.
* **Firewall bypass testing** — testing connectivity and traffic routing through different network paths.
* **Multi-location testing** — testing applications and services from different geographic locations.
* **Web scraping systems** — routing and distributing requests through proxy servers in different locations.


## Features

* HTTP `CONNECT` proxy
* SOCKS5 proxy
* SOCKS5 `CONNECT`
* SOCKS5 `UDP ASSOCIATE`
* HTTP and SOCKS5 over TLS
* Username/password authentication
* Domain and wildcard domain routing
* IPv4 and IPv6 CIDR routing
* DNS-based CIDR routing
* Direct TCP connections
* Broker-based TCP and UDP connections
* Persistent WebSocket connection to the Broker
* Multiple Worker support
* Admin Panel integration

---

## Quick Start

> **⚠️ Development / Testing Only**
>
> The setup below is intended for local testing and initial evaluation. **Do not expose this setup directly to the Internet in production.**

The easiest way to get started is to run the complete stack locally, register a Worker, create a Proxy account, configure routing, and test the Proxy with `curl`.

### 1. Configure the main stack

Copy the example environment file:

```bash
cp .env.example .env
```

Fill in `.env` with the actual values required for your environment.

### 2. Start the main stack

```bash
docker-compose up -d --build
```

### 3. Start a Worker

Create the Worker environment file:

```bash
cp .env.worker.example .env.worker
```

Fill in `.env.worker` with the actual values required for your environment.

Start the Worker:

```bash
docker-compose -f docker-compose.worker.yml up -d --build
```

### 4. Test the Proxy

Before configuring routing, you can verify that the Proxy is reachable and accepts the test credentials.

#### HTTP / HTTPS

```bash
curl --proxy http://test-proxy:test-proxy-password@127.0.0.1:8787 https://google.com
```

#### SOCKS5

```bash
curl --proxy socks5h://test-proxy:test-proxy-password@127.0.0.1:8888 https://google.com
```

If both commands complete successfully, the Proxy is running and accepting connections with the test credentials.

### 5. Configure the Proxy

Open the Admin Panel at the configured FRONT_PORT:

1. Register the Worker.
2. Create one or more Proxy accounts.
3. Create the required groups.
4. Configure the routing rules.

After the configuration is complete, repeat the Proxy tests from step 4 to verify that the selected routing configuration is applied.

For troubleshooting, check the service logs:

```bash
docker-compose logs -f proxy
docker-compose logs -f broker
docker-compose logs -f worker
```

If this works, you have a running Proxy, a connected Worker, and a working routing configuration.

## Production Setup

The Quick Start setup is intended for local development and testing.

**Do not use it as-is in production.**

> **⚠️ Important:** Before deploying to production, change all default Proxy usernames and passwords. In particular, do not use the default test credentials such as `test-proxy:test-proxy-password` in a production environment.

For production, put Nginx in front of the Proxy/Broker endpoint and expose the service through a domain with TLS.

### Nginx

A minimal configuration looks like this:

```nginx
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen 80;
    server_name example.com;

    location ^~ /.well-known/acme-challenge/ {
        default_type "text/plain";
        root /home/www/letsencrypt;
    }
}

server {
    listen 443 ssl;
    server_name example.com;

    ssl_certificate /etc/letsencrypt/live/example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:<FRONT_PORT>;

        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;

        proxy_set_header Host $host;

        proxy_read_timeout 3600;
        proxy_send_timeout 3600;
    }
}
```

Replace:

* `example.com` with your domain.
* `<FRONT_PORT>` with the port exposed by the Admin Panel frontend.
* The certificate paths with the paths used by your TLS setup.

The WebSocket headers are required for persistent communication between the Proxy and Broker.

### TLS Certificates

For production, it is recommended to use **Let's Encrypt certificates** or certificates issued by another trusted Certificate Authority.

Configure the certificate paths in the application's TLS environment variables:

```env
TLS_CERT_FILE=/app/certs/live/example.com/fullchain.pem
TLS_KEY_FILE=/app/certs/live/example.com/privkey.pem
CERTS_DIR=/etc/letsencrypt/
```

Replace `example.com` with your actual domain.

Make sure that the certificate files are mounted into the container at the paths specified by `TLS_CERT_FILE` and `TLS_KEY_FILE`, and that `CERTS_DIR` points to the directory containing your TLS certificates.

If you use certificates issued by another Certificate Authority, configure the corresponding certificate and key paths instead.

### UDP Relay

If UDP relay support is required, configure `UDP_RELAY_HOST` to an IP address that is reachable by the Proxy clients.

The configured host must be accessible on the UDP port range **55000–60000**.

For example:

```env
UDP_RELAY_HOST=<PUBLIC_IP>
```

Replace `<PUBLIC_IP>` with the IP address that clients can use to reach the UDP relay.

Make sure that the corresponding UDP ports **55000–60000** are allowed through the firewall and are reachable from the clients.

### Remote Workers

Once the production endpoint is available, additional Workers can be deployed on remote machines.

Configure each Worker to connect to the public Broker endpoint:

```env
BROKER_ADDR=wss://example.com
```

Then start the Worker:

```bash
docker-compose -f docker-compose.worker.yml up -d --build
```

Repeat this on each remote machine.

### Worker Groups and Routing

Register the remote Workers in the Admin Panel and assign them to the required groups.

Then configure routing rules to send traffic to the desired groups.

For example:

```text
Client
  │
  ▼
Proxy
  │
  ├── Direct
  │
  └── Group: Germany
          │
          ▼
       Worker DE
```

Once the routing rules are configured, the same Proxy endpoint can route different traffic through Workers running on different machines and locations.

---

## Architecture

The Proxy is the entry point for client traffic.

```text
                         ┌──────────────┐
                         │    Client    │
                         └──────┬───────┘
                                │
                         HTTP / SOCKS5
                                │
                                ▼
                         ┌──────────────┐
                         │    Proxy     │
                         └──────┬───────┘
                                │
                       ┌────────┴────────┐
                       │                 │
                    DIRECT             BROKER
                       │                 │
                       │                 ▼
                       │             ┌─────────┐
                       │             │ Broker  │
                       │             └────┬────┘
                       │                  │
                       │                  ▼
                       │             ┌─────────┐
                       │             │ Worker  │
                       │             └────┬────┘
                       │                  │
                       └────────┬─────────┘
                                │
                                ▼
                           Destination
```

The Proxy uses:

* **PostgreSQL** for Proxy authentication.
* **Redis** for routing configuration.
* **Broker** for traffic that must be forwarded through a Worker.
* **Admin Panel** for managing Proxy accounts, Workers, groups, and routing rules.

---

## How Routing Works

The Proxy resolves a destination to a **group**.

Groups are managed through the Admin Panel. A group determines how traffic should be handled.

The routing process is approximately:

```text
Client request
      │
      ▼
Domain rule?
      │
      ├── Yes ──> Group
      │
      └── No
           │
           ▼
      Wildcard rule?
           │
           ├── Yes ──> Group
           │
           └── No
                │
                ▼
           CIDR rule?
                │
                ├── Yes ──> Group
                │
                └── No
                     │
                     ▼
                Default group
```

### Domain Rules

Exact domain rules match normalized hostnames.

For example:

```text
example.com -> Germany
```

The hostname is normalized by:

* converting it to lowercase;
* trimming whitespace;
* removing a trailing dot.

### Wildcard Rules

Wildcard rules use the following format:

```text
*.example.com
```

They match subdomains:

```text
api.example.com
www.example.com
foo.bar.example.com
```

but not the base domain:

```text
example.com
```

### CIDR Rules

Both IPv4 and IPv6 CIDRs are supported.

Examples:

```text
10.0.0.0/8
10.10.0.0/16
2001:db8::/32
```

When multiple CIDR rules match, the most specific prefix takes precedence.

### DNS-based CIDR Routing

When a hostname is used as the destination, the Proxy can resolve it and apply CIDR routing to the resulting IP address.

```text
example.internal
       │
       ▼
      DNS
       │
       ▼
  10.10.20.30
       │
       ▼
  10.0.0.0/8
       │
       ▼
     Group
```

### Default Route

If no specific domain or CIDR rule matches, the Proxy uses the configured default group.

The default route is stored in Redis and can be changed without restarting the Proxy.

---

## Direct Routing

A group configured as `DEFAULT_GROUP_NAME` is treated by the Proxy as the direct-routing group.

Traffic assigned to this group is connected directly to the destination without going through a Worker.

The Proxy blocks direct connections to:

* Loopback addresses
* Private addresses
* Link-local addresses
* Unspecified addresses
* Multicast addresses

This prevents the direct Proxy path from being used to access local or private network resources.

---

## Broker Routing

Traffic assigned to a non-direct group is forwarded through the Broker and an appropriate Worker.

```text
Proxy
  │
  │ WebSocket
  ▼
Broker
  │
  ▼
Worker
  │
  ▼
Destination
```

The Proxy maintains a persistent WebSocket connection to the Broker and multiplexes multiple client sessions over it.

Broker connections are established when required by the first request using a Broker-based route.

---

## Supported Protocols

### HTTP CONNECT

The Proxy supports HTTP `CONNECT`.

Example:

```http
CONNECT example.com:443 HTTP/1.1
Host: example.com:443
Proxy-Authorization: Basic <credentials>
```

A successful connection returns:

```http
HTTP/1.1 200 Connection Established
```

Unsupported HTTP methods return:

```http
HTTP/1.1 501 Not Implemented
```

### SOCKS5

The Proxy supports:

* Username/password authentication
* `CONNECT`
* `UDP ASSOCIATE`
* IPv4
* IPv6
* Domain names

Example:

```bash
curl \
  --proxy socks5h://<USERNAME>:<PASSWORD>@127.0.0.1:<PROXY_PORT> \
  https://example.com
```

### TLS

The Proxy can accept TLS connections and then handle HTTP or SOCKS5 traffic inside the TLS connection.

The minimum supported TLS version is TLS 1.2.

---

## Authentication

Proxy authentication is backed by PostgreSQL.

HTTP clients use:

```http
Proxy-Authorization: Basic <base64(username:password)>
```

SOCKS5 clients use username/password authentication.

Passwords are stored as PBKDF2-SHA256 hashes.

Proxy users are periodically synchronized from PostgreSQL, so changes made to Proxy accounts are applied without restarting the Proxy.

---

## Docker

Build and start the Proxy:

```bash
docker-compose up -d --build
```

View logs:

```bash
docker-compose logs -f proxy
```

Restart the Proxy:

```bash
docker-compose restart proxy
```

Stop the stack:

```bash
docker-compose down
```

---

## Troubleshooting

### Proxy does not start

Check the logs:

```bash
docker-compose logs proxy
```

Verify:

* PostgreSQL is available.
* Redis is available.
* Required environment variables are configured.
* TLS certificate and key exist.
* The configured ports are available.

### Authentication fails

Check:

* The Proxy account exists in the Admin Panel.
* The username and password are correct.
* PostgreSQL is available.
* The Proxy has successfully synchronized users.

### Worker is not available

Check:

```bash
docker-compose logs worker
docker-compose logs broker
docker-compose logs proxy
```

Verify:

* `BROKER_ADDR`
* `PROXY_TOKEN`
* `RANDOM_ENDPOINT_SECRET`
* Worker registration in the Admin Panel
* Worker group assignment

### Routing does not work

Check:

1. The Worker is registered.
2. The Worker belongs to the expected group.
3. The routing rule points to the correct group.
4. The destination matches the expected domain or CIDR rule.
5. Redis is available.
6. The Proxy has reloaded the latest routing configuration.

---

## Security

The Quick Start configuration is intended for local testing only.

For production:

* Use TLS.
* Put Nginx or another reverse proxy in front of the public endpoint.
* Use strong Proxy and Worker tokens.
* Do not commit `.env` files containing secrets.
* Do not expose PostgreSQL or Redis publicly.
* Protect the Admin Panel.
* Use secure TLS certificates and private keys.
* Restrict access to internal services whenever possible.
