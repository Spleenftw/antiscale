# Antiscale: Self-Hosted Zero Trust Mesh 🌐

Antiscale is a lightweight, open-source competitor to Tailscale, designed to give you 100% control over your WireGuard routing mesh. It consists of two deployable components:

1. **The Controller (Backend+Panel):** A central Go-based API and Premium React Dashboard that orchestrates CGNAT IP assignment, generates MagicDNS flower names, evaluates JSON network ACLs, and authenticates admins via GitHub SSO.
2. **The Client Agent:** A containerized Go daemon that runs on your endpoint devices, managing WireGuard keys and automatically registering with the Controller based on your defined routing intent.

---

## 🛠️ 1. Deploying the Controller

The Controller should be deployed on a central VPS with a public IP. It requires GitHub OAuth credentials so the Admin Dashboard remains secure.

### Prerequisite: GitHub OAuth App
1. Go to **GitHub Settings -> Developer Settings -> OAuth Apps -> New OAuth App**.
2. Set the **Homepage URL** to your server's URL (e.g., `http://my-vps-ip:8080`).
3. Set the **Authorization callback URL** to `http://my-vps-ip:8080/api/auth/github/callback`.
4. Copy the generated `Client ID` and `Client Secret`.

### Server Deployment

Create a `.env` file in the root directory (next to `docker-compose.yml`) and insert your GitHub credentials:

```env
# Controller .env configuration
GITHUB_CLIENT_ID=your_github_client_id_here
GITHUB_CLIENT_SECRET=your_github_client_secret_here
```

Spin up the control server:
```bash
docker-compose up -d --build
```
*   The **Administration Dashboard** is now available on port `80` (e.g. `http://localhost` if developing locally).
*   The **Mesh API** is now available on port `8080`.

---

## 🚀 2. Deploying a Client Device

The Client Agent runs on the devices you want to connect into your mesh. Because it manipulates kernel network interfaces, it must run with elevated permissions.

### Prerequisite: Auth Key
Log into your new Antiscale Dashboard, navigate to **Pre-Auth Keys**, and generate a key (e.g., `antskey-34fdf3a4`).

### Client Deployment

Extract `docker-compose.client.yml` and place it on your edge device. Next to it, create a `.env` file defining the client's routing capabilities:

```env
# Client .env configuration
CONTROLLER_URL=http://<YOUR_CONTROLLER_IP>:8080
NODE_NAME=ubuntu-worker

# Authentication
AUTH_KEY=antskey-34fdf3a4

# Route Advertising (Optional)
# E.g., to act as a subnet router or default internet exit-node
ADVERTISE_ROUTES=192.168.1.0/24,0.0.0.0/0

# Route Acceptance (Optional)
# If true, this node will try to reach other subnets advertised by peers
ACCEPT_ROUTES=true
```

Run the container using the client template:
```bash
docker-compose -f docker-compose.client.yml up -d
```

### What happens next?
The client immediately contacts the Controller, uses the `AUTH_KEY` to link itself your specific User Identity, calculates a `100.64.0.x` IP and MagicDNS name (e.g. `peony-dahlia`), and appears live in your Dashboard!

---

## 🛡️ Network Controls (ACLs)

From the Dashboard, navigate to the **Access Config** tab and modify the HuJSON mesh rules. 
When clients hit the network sync endpoint, the Controller parses your exact policies and acts as a strict firewall—entirely omitting forbidden peers from the local network map. 
