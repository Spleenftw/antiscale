# Antiscale: Self-Hosted Zero Trust Mesh 🌐

Antiscale is a lightweight, open-source competitor to Tailscale, designed to give you 100% control over your WireGuard routing mesh. It consists of two deployable components:

1. **The Controller (Backend+Panel):** A central Go-based API and Premium React Dashboard that orchestrates CGNAT IP assignment, generates MagicDNS flower names, evaluates JSON network ACLs, and authenticates admins via GitHub SSO.
2. **The Client Agent:** A containerized Go daemon that runs on your endpoint devices, managing WireGuard keys and automatically registering with the Controller based on your defined routing intent.

---

## 🛠️ 1. Deploying the Controller

The Controller should be deployed on a central VPS with a public IP. It requires GitHub OAuth credentials so the Admin Dashboard remains secure.

### Prerequisite: GitHub OAuth App
1. Go to **GitHub Settings -> Developer Settings -> OAuth Apps -> New OAuth App**.
2. Set the **Homepage URL** to your server's URL (e.g., `http://my-vps-ip`).
3. Set the **Authorization callback URL** to `http://my-vps-ip:8080/api/auth/github/callback`.
4. Copy the generated `Client ID` and `Client Secret`.

### Server Deployment

Use the following `docker-compose.yml` to deploy the controller using the pre-built DockerHub images:

```yaml
services:
  controller:
    image: spleenftw/antiscale-controller:latest
    ports:
      - "8080:8080"
    volumes:
      - data_volume:/app/data
    restart: unless-stopped
    environment:
      - PORT=8080
      - DB_PATH=/app/data/antiscale.db
      - GITHUB_CLIENT_ID=${GITHUB_CLIENT_ID}
      - GITHUB_CLIENT_SECRET=${GITHUB_CLIENT_SECRET}

  frontend:
    image: spleenftw/antiscale-ui:latest
    ports:
      - "80:80"
    restart: unless-stopped
    depends_on:
      - controller

volumes:
  data_volume:
```

Create a `.env` file next to it:
```env
GITHUB_CLIENT_ID=your_github_client_id_here
GITHUB_CLIENT_SECRET=your_github_client_secret_here
```

Spin up the control server:
```bash
docker compose up -d
```
*   The **Administration Dashboard** is now on port `80`.
*   The **Mesh API** is now on port `8080`.

---

## 🚀 2. Deploying a Client Device

The Client Agent runs on the devices you want to connect into your mesh. Because it manipulates kernel network interfaces, it must run with elevated permissions.

Log into your Dashboard, navigate to **Pre-Auth Keys**, and generate a key (e.g., `antskey-34fdf3a4`).

You can deploy the client using **Docker Run** (Easiest) or **Docker Compose**.

### Option A: Docker Run (Easiest)

Run this single command on your edge device, replacing the variables:

```bash
sudo docker run -d \
  --name antiscale-client \
  --net=host \
  --cap-add=NET_ADMIN \
  --cap-add=SYS_MODULE \
  -v /dev/net/tun:/dev/net/tun \
  -v /lib/modules:/lib/modules \
  -e CONTROLLER_URL=http://<YOUR_CONTROLLER_IP>:8080 \
  -e NODE_NAME=ubuntu-worker \
  -e AUTH_KEY=antskey-34fdf3a4 \
  spleenftw/antiscale-client:latest
```

### Option B: Docker Compose

Create a `docker-compose.client.yml`:

```yaml
services:
  antiscale-client:
    image: spleenftw/antiscale-client:latest
    container_name: antiscale-client
    environment:
      - CONTROLLER_URL=http://<YOUR_CONTROLLER_IP>:8080
      - NODE_NAME=ubuntu-worker
      - AUTH_KEY=antskey-34fdf3a4
    cap_add:
      - NET_ADMIN
      - SYS_MODULE
    network_mode: "host"
    restart: unless-stopped
    volumes:
      - /dev/net/tun:/dev/net/tun
      - /lib/modules:/lib/modules
```

Deploy it:
```bash
sudo docker compose -f docker-compose.client.yml up -d
```

### What happens next?
The client calculates a `100.64.0.x` IP and MagicDNS name (e.g. `peony-dahlia`), installs WireGuard locally, syncs peers, and appears live in your Dashboard!
